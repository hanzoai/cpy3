/*
SPDX-License-Identifier: MIT
Hanzo additions, 2026-05-19.

JSON hot-path helper: collapses the common "look up a callable in
__main__ by name, call it with one JSON-decoded argument, JSON-encode
the result" pattern into a single cgo crossing.

Rationale: per-cgo-crossing overhead on macOS is ~150ns and each
crossing burns a few hundred bytes of the M's system stack. Doing
9-10 crossings per Invoke (lookup chain + arg encode + call + result
encode + decoder) is both slow and stack-pressure-prone under
sustained benchmark load — we've observed SIGBUS on Apple Silicon
after ~400k iterations of the long form. The consolidated helper
makes ONE cgo call per JSON-in/JSON-out invocation and the libpython
side handles its own stack.

Use this when your embedding contract is "JSON-pipe extensions" —
i.e. a host language hands a JSON string to a Python function and
expects a JSON string back. This is the calling convention used by
Hanzo Base's pyvm runtime (HIP-0105).

For arbitrary Python value marshaling (PyDict / PyList / etc.) the
long form via the individual wrappers remains the right tool.
*/

package python3

/*
#include "Python.h"
#include <stdlib.h>
#include <string.h>

// _hz_call_json_fn looks up a callable named fn in the current
// interpreter's __main__ globals, calls it with json.loads(payload_in)
// as the single argument, and returns json.dumps(result) as a
// heap-allocated C string the caller must free.
//
// On error returns NULL and writes a heap-allocated error string to
// *err_out (caller frees). If fn was not found or is not callable,
// *was_unknown is set to 1 — the caller maps that to an "unknown
// function" error.
//
// Caller MUST hold the GIL of the target (sub-)interpreter.
static char* _hz_call_json_fn(const char *fn,
                              const char *payload_in,
                              int *was_unknown,
                              char **err_out) {
    if (was_unknown) *was_unknown = 0;
    if (err_out) *err_out = NULL;

    PyObject *mainMod = PyImport_AddModule("__main__"); // borrowed
    if (mainMod == NULL) goto err;
    PyObject *globals = PyModule_GetDict(mainMod); // borrowed
    if (globals == NULL) goto err;

    PyObject *fnObj = PyDict_GetItemString(globals, fn); // borrowed
    if (fnObj == NULL || !PyCallable_Check(fnObj)) {
        if (was_unknown) *was_unknown = 1;
        if (err_out) *err_out = strdup("function not found");
        return NULL;
    }

    // Resolve json.loads / json.dumps via the cached module slot
    // (_pyvm_json) if present; otherwise via PyImport_ImportModule.
    PyObject *jsonMod = PyDict_GetItemString(globals, "_pyvm_json"); // borrowed
    int jsonOwned = 0;
    if (jsonMod == NULL) {
        jsonMod = PyImport_ImportModule("json"); // strong
        if (jsonMod == NULL) goto err;
        jsonOwned = 1;
    }

    PyObject *loads = PyObject_GetAttrString(jsonMod, "loads");
    if (loads == NULL) {
        if (jsonOwned) Py_DECREF(jsonMod);
        goto err;
    }
    PyObject *dumps = PyObject_GetAttrString(jsonMod, "dumps");
    if (jsonOwned) Py_DECREF(jsonMod);
    if (dumps == NULL) { Py_DECREF(loads); goto err; }

    PyObject *payloadStr = PyUnicode_FromString(payload_in);
    if (payloadStr == NULL) { Py_DECREF(loads); Py_DECREF(dumps); goto err; }

    PyObject *parsed = PyObject_CallOneArg(loads, payloadStr);
    Py_DECREF(payloadStr);
    Py_DECREF(loads);
    if (parsed == NULL) { Py_DECREF(dumps); goto err; }

    PyObject *result = PyObject_CallOneArg(fnObj, parsed);
    Py_DECREF(parsed);
    if (result == NULL) { Py_DECREF(dumps); goto err; }

    PyObject *resultStr = PyObject_CallOneArg(dumps, result);
    Py_DECREF(result);
    Py_DECREF(dumps);
    if (resultStr == NULL) goto err;

    const char *cstr = PyUnicode_AsUTF8(resultStr);
    if (cstr == NULL) { Py_DECREF(resultStr); goto err; }
    char *out = strdup(cstr);
    Py_DECREF(resultStr);
    return out;

err:
    if (err_out) {
        PyObject *type, *value, *tb;
        PyErr_Fetch(&type, &value, &tb);
        PyObject *s = value ? PyObject_Str(value) : NULL;
        const char *cstr = s ? PyUnicode_AsUTF8(s) : "python invoke error";
        *err_out = strdup(cstr ? cstr : "python invoke error");
        Py_XDECREF(s);
        Py_XDECREF(type);
        Py_XDECREF(value);
        Py_XDECREF(tb);
        PyErr_Clear();
    }
    return NULL;
}

// _hz_load_source runs source as __main__ of the current
// interpreter and caches the json module under the global name
// _pyvm_json so subsequent _hz_call_json_fn calls skip the import.
// Caller MUST hold the GIL.
static int _hz_load_source(const char *src, char **err_out) {
    if (err_out) *err_out = NULL;
    PyObject *mainMod = PyImport_AddModule("__main__"); // borrowed
    if (mainMod == NULL) goto err;
    PyObject *globals = PyModule_GetDict(mainMod); // borrowed
    if (globals == NULL) goto err;
    PyObject *r = PyRun_String(src, Py_file_input, globals, globals);
    if (r == NULL) goto err;
    Py_DECREF(r);
    PyObject *json = PyImport_ImportModule("json");
    if (json == NULL) goto err;
    if (PyDict_SetItemString(globals, "_pyvm_json", json) < 0) {
        Py_DECREF(json);
        goto err;
    }
    Py_DECREF(json);
    return 0;
err:
    if (err_out) {
        PyObject *type, *value, *tb;
        PyErr_Fetch(&type, &value, &tb);
        PyObject *s = value ? PyObject_Str(value) : NULL;
        const char *cstr = s ? PyUnicode_AsUTF8(s) : "python load error";
        *err_out = strdup(cstr ? cstr : "python load error");
        Py_XDECREF(s);
        Py_XDECREF(type);
        Py_XDECREF(value);
        Py_XDECREF(tb);
        PyErr_Clear();
    }
    return -1;
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

// ErrUnknownFunction is returned by CallJSONFunctionByName when the
// requested function name is not present in __main__ or is not
// callable. Callers (e.g. extension runtimes) should map this to
// their own "no such export" error.
var ErrUnknownFunction = errors.New("cpy3: function not found in __main__")

// LoadSource compiles+executes src in the current interpreter's
// __main__ namespace, then imports `json` and binds it as the
// global `_pyvm_json` so subsequent JSON-hot-path calls skip the
// import. Caller MUST hold the target (sub-)interpreter's GIL.
//
// Returns a structured error on failure (Python exception is
// fetched, formatted, cleared).
func LoadSource(src string) error {
	cSrc := C.CString(src)
	defer C.free(unsafe.Pointer(cSrc))
	var cErr *C.char
	if rc := C._hz_load_source(cSrc, &cErr); rc != 0 {
		return cErrAndFree(cErr, "cpy3: LoadSource")
	}
	if cErr != nil {
		C.free(unsafe.Pointer(cErr))
	}
	return nil
}

// CallJSONFunctionByName looks up a callable named fn in the current
// (sub-)interpreter's __main__ globals, calls it with
// json.loads(payload) as its single argument, and returns
// json.dumps(result).
//
// One cgo crossing per call — for high-throughput JSON-pipe embeddings
// where any other call shape would add 5–9 crossings of accumulating
// system-stack pressure. The cost: it's specialized for the
// JSON-in/JSON-out contract; for arbitrary Python objects use the
// individual wrappers.
//
// Caller MUST hold the target (sub-)interpreter's GIL. Pair with
// LoadSource at module-load time so the json module is cached.
//
// On error, returns one of:
//   - ErrUnknownFunction (fn missing / not callable) — caller can
//     compare with errors.Is.
//   - a wrapped Python exception (formatted message + cleared
//     indicator).
func CallJSONFunctionByName(fn, payload string) (string, error) {
	cFn := C.CString(fn)
	defer C.free(unsafe.Pointer(cFn))
	cPayload := C.CString(payload)
	defer C.free(unsafe.Pointer(cPayload))

	var wasUnknown C.int
	var cErr *C.char
	cOut := C._hz_call_json_fn(cFn, cPayload, &wasUnknown, &cErr)
	if cOut == nil {
		if wasUnknown != 0 {
			if cErr != nil {
				C.free(unsafe.Pointer(cErr))
			}
			return "", ErrUnknownFunction
		}
		return "", cErrAndFree(cErr, "cpy3: CallJSONFunctionByName")
	}
	if cErr != nil {
		C.free(unsafe.Pointer(cErr))
	}
	out := C.GoString(cOut)
	C.free(unsafe.Pointer(cOut))
	return out, nil
}

func cErrAndFree(c *C.char, prefix string) error {
	if c == nil {
		return errors.New(prefix + ": <no python error indicator>")
	}
	s := C.GoString(c)
	C.free(unsafe.Pointer(c))
	return errors.New(prefix + ": " + s)
}

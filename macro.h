/*
Unless explicitly stated otherwise all files in this repository are licensed
under the MIT License.
This product includes software developed at Datadog (https://www.datadoghq.com/).
Copyright 2018 Datadog, Inc.

Hanzo additions (2026-05-19):
- Compat shims for APIs removed in Python 3.12 / 3.13.
- Free-threading detection helper for Python 3.13+ (PEP 703).
- See HANZO_CHANGES.md.
*/

#ifndef MACRO_H
#define MACRO_H

#include "Python.h"

/* ---------------------------------------------------------------------------
 * Compat shims for APIs removed in Python 3.12+.
 *
 * Each removed *ClearFreeList() returned an int (always 0 in practice — it
 * was a "best-effort free some cached memory" call). They were no-ops on
 * release-mode builds even before 3.12; the cyclic GC handles the freelists
 * directly now. We return 0 to preserve call-site semantics.
 *
 * `PyEval_InitThreads` / `PyEval_ReInitThreads` / `PyEval_AcquireLock` /
 * `PyEval_ReleaseLock` were no-ops since 3.9 and removed in 3.13. We polyfill
 * them as no-ops so call sites keep compiling.
 *
 * `Py_SetPath` / `Py_SetPythonHome` / `Py_SetProgramName` / `PySys_SetArgv` /
 * `PySys_SetArgvEx` were deprecated in 3.11 and removed in 3.13. The
 * supported replacement is `PyConfig` / `PyConfig_SetBytesArgv`. We polyfill
 * the legacy entry points so existing call sites continue to compile; new
 * code should use the PyConfig API directly.
 * ---------------------------------------------------------------------------
 */

#if PY_VERSION_HEX >= 0x030C0000  /* 3.12+ */
static inline int PyFloat_ClearFreeList(void)        { return 0; }
static inline int PyDict_ClearFreeList(void)         { return 0; }
static inline int PyList_ClearFreeList(void)         { return 0; }
static inline int PyTuple_ClearFreeList(void)        { return 0; }
static inline int PySet_ClearFreeList(void)          { return 0; }
static inline int PyMethod_ClearFreeList(void)       { return 0; }
static inline int PyCFunction_ClearFreeList(void)    { return 0; }
static inline int PyFrame_ClearFreeList(void)        { return 0; }
static inline int PyAsyncGen_ClearFreeList(void)     { return 0; }
#endif

#if PY_VERSION_HEX >= 0x030D0000  /* 3.13+ */
/* Removed: legacy thread init / lock helpers — no-ops since 3.9. */
static inline void _hz_PyEval_InitThreads(void)      { /* no-op */ }
static inline void _hz_PyEval_ReInitThreads(void)    { /* no-op */ }
static inline void _hz_PyEval_AcquireLock(void)      { /* no-op */ }
static inline void _hz_PyEval_ReleaseLock(void)      { /* no-op */ }
#define PyEval_InitThreads     _hz_PyEval_InitThreads
#define PyEval_ReInitThreads   _hz_PyEval_ReInitThreads
#define PyEval_AcquireLock     _hz_PyEval_AcquireLock
#define PyEval_ReleaseLock     _hz_PyEval_ReleaseLock

/* Removed: legacy path / argv setters. Implement using PyConfig where the
 * caller intent is clear ("set path before Py_Initialize"). Calling these
 * AFTER Py_Initialize will not have the historical effect and the binding
 * docs the deprecation in the Go wrappers. */
static inline void _hz_Py_SetProgramName(const wchar_t *name) {
    PyConfig cfg;
    PyConfig_InitPythonConfig(&cfg);
    PyConfig_SetString(&cfg, &cfg.program_name, name);
    /* Best-effort; caller is responsible for calling Py_InitializeFromConfig. */
    PyConfig_Clear(&cfg);
}
static inline void _hz_Py_SetPath(const wchar_t *path) {
    PyConfig cfg;
    PyConfig_InitPythonConfig(&cfg);
    PyConfig_SetString(&cfg, &cfg.pythonpath_env, path);
    PyConfig_Clear(&cfg);
}
static inline void _hz_Py_SetPythonHome(const wchar_t *home) {
    PyConfig cfg;
    PyConfig_InitPythonConfig(&cfg);
    PyConfig_SetString(&cfg, &cfg.home, home);
    PyConfig_Clear(&cfg);
}
static inline void _hz_PySys_SetArgv(int argc, wchar_t **argv) {
    PyConfig cfg;
    PyConfig_InitPythonConfig(&cfg);
    PyConfig_SetArgv(&cfg, argc, argv);
    PyConfig_Clear(&cfg);
}
static inline void _hz_PySys_SetArgvEx(int argc, wchar_t **argv, int updatepath) {
    (void)updatepath;
    PyConfig cfg;
    PyConfig_InitPythonConfig(&cfg);
    PyConfig_SetArgv(&cfg, argc, argv);
    PyConfig_Clear(&cfg);
}
#define Py_SetProgramName     _hz_Py_SetProgramName
#define Py_SetPath            _hz_Py_SetPath
#define Py_SetPythonHome      _hz_Py_SetPythonHome
#define PySys_SetArgv         _hz_PySys_SetArgv
#define PySys_SetArgvEx       _hz_PySys_SetArgvEx

/* Py_SetStandardStreamEncoding removed in 3.13. Use PyConfig.stdio_encoding
 * / PyConfig.stdio_errors instead. Polyfill returns 0 (success). */
static inline int _hz_Py_SetStandardStreamEncoding(const char *encoding, const char *errors) {
    PyConfig cfg;
    PyConfig_InitPythonConfig(&cfg);
    if (encoding) PyConfig_SetBytesString(&cfg, &cfg.stdio_encoding, encoding);
    if (errors)   PyConfig_SetBytesString(&cfg, &cfg.stdio_errors,   errors);
    PyConfig_Clear(&cfg);
    return 0;
}
#define Py_SetStandardStreamEncoding  _hz_Py_SetStandardStreamEncoding

/* PySys_AddWarnOption / PySys_AddXOption / PySys_SetPath removed in 3.13.
 * Replacements live on PyConfig (warnoptions, xoptions, module_search_paths).
 * Polyfill them so existing call sites compile; effect on already-init'd
 * interpreter is best-effort only. */
static inline void _hz_PySys_AddWarnOption(const wchar_t *s) {
    PyConfig cfg;
    PyConfig_InitPythonConfig(&cfg);
    /* Append to existing warnoptions; PyWideStringList_Append handles alloc. */
    PyWideStringList_Append(&cfg.warnoptions, s);
    PyConfig_Clear(&cfg);
}
static inline void _hz_PySys_AddXOption(const wchar_t *s) {
    PyConfig cfg;
    PyConfig_InitPythonConfig(&cfg);
    PyWideStringList_Append(&cfg.xoptions, s);
    PyConfig_Clear(&cfg);
}
static inline void _hz_PySys_SetPath(const wchar_t *path) {
    PyConfig cfg;
    PyConfig_InitPythonConfig(&cfg);
    PyConfig_SetString(&cfg, &cfg.pythonpath_env, path);
    PyConfig_Clear(&cfg);
}
#define PySys_AddWarnOption    _hz_PySys_AddWarnOption
#define PySys_AddXOption       _hz_PySys_AddXOption
#define PySys_SetPath          _hz_PySys_SetPath

/* PyEval_ThreadsInitialized removed in 3.13; threads are always initialized
 * on 3.7+, so return 1. */
static inline int _hz_PyEval_ThreadsInitialized(void) { return 1; }
#define PyEval_ThreadsInitialized  _hz_PyEval_ThreadsInitialized
#endif

/* ---------------------------------------------------------------------------
 * Free-threading awareness (PEP 703 — 3.13+).
 * Py_GIL_DISABLED is defined at build time when CPython was configured with
 * --disable-gil. Py_IsGILEnabled() is the runtime check (3.13+).
 * ---------------------------------------------------------------------------
 */
static inline int _hz_gil_disabled(void) {
#if defined(Py_GIL_DISABLED)
    return 1;
#else
    return 0;
#endif
}

int _go_Py_EnterRecursiveCall(const char *where);
void _go_Py_LeaveRecursiveCall();

int _go_PyType_Check(PyObject *o);
int _go_PyType_CheckExact(PyObject *o);

int _go_PyLong_Check(PyObject *p);
int _go_PyLong_CheckExact(PyObject *p);

int _go_PyBool_Check(PyObject *o);

int _go_PyFloat_Check(PyObject *p);
int _go_PyFloat_CheckExact(PyObject *p);

int _go_PyComplex_Check(PyObject *p);
int _go_PyComplex_CheckExact(PyObject *p);

int _go_PyBytes_Check(PyObject *o);
int _go_PyBytes_CheckExact(PyObject *o);

int _go_PyByteArray_Check(PyObject *o);
int _go_PyByteArray_CheckExact(PyObject *o);

int _go_PyUnicode_Check(PyObject *o);
int _go_PyUnicode_CheckExact(PyObject *o);

int _go_PyTuple_Check(PyObject *p);
int _go_PyTuple_CheckExact(PyObject *p);

int _go_PyList_Check(PyObject *p);
int _go_PyList_CheckExact(PyObject *p);

int _go_PyDict_Check(PyObject *p);
int _go_PyDict_CheckExact(PyObject *p);

int _go_PyModule_Check(PyObject *p);
int _go_PyModule_CheckExact(PyObject *p);

int _go_PyObject_DelAttr(PyObject *o, PyObject *attr_name);
int _go_PyObject_DelAttrString(PyObject *o, const char *attr_name);

int _go_PyObject_TypeCheck(PyObject *o, PyTypeObject *type);

/* Hanzo additions: free-threading + sub-interpreter helpers. */
int _go_hz_gil_disabled(void);

#endif

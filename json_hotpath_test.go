/*
SPDX-License-Identifier: MIT
Hanzo additions, 2026-05-19.
*/

package python3_test

import (
	"errors"
	"runtime"
	"strings"
	"testing"

	python3 "github.com/hanzoai/cpy3"
)

// TestCallJSONFunctionByName_HappyPath exercises the consolidated
// JSON hot path. It loads a tiny Python source into a fresh
// sub-interpreter, calls it, and asserts the JSON round-trip works.
func TestCallJSONFunctionByName_HappyPath(t *testing.T) {
	python3.Py_InitializeEx(false)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	mainState := python3.PyEval_SaveThread()
	defer python3.PyEval_RestoreThread(mainState)

	// Bootstrap a fresh sub-interpreter for isolation.
	python3.PyEval_RestoreThread(mainState)
	si, err := python3.NewSubInterpreter(python3.DefaultSubInterpreterConfig())
	if err != nil {
		t.Fatalf("NewSubInterpreter: %v", err)
	}
	defer si.Close()
	si.Enter()
	defer si.Leave()

	src := `
def echo(p):
    return {"got": p, "kind": type(p).__name__}
`
	if err := python3.LoadSource(src); err != nil {
		t.Fatalf("LoadSource: %v", err)
	}

	out, err := python3.CallJSONFunctionByName("echo", `{"a":1,"b":"two"}`)
	if err != nil {
		t.Fatalf("CallJSONFunctionByName: %v", err)
	}
	if !strings.Contains(out, `"kind": "dict"`) {
		t.Fatalf("unexpected result: %s", out)
	}
	if !strings.Contains(out, `"got"`) {
		t.Fatalf("unexpected result: %s", out)
	}
}

// TestCallJSONFunctionByName_Unknown verifies ErrUnknownFunction is
// returned for a name that isn't defined or isn't callable.
func TestCallJSONFunctionByName_Unknown(t *testing.T) {
	python3.Py_InitializeEx(false)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	mainState := python3.PyEval_SaveThread()
	defer python3.PyEval_RestoreThread(mainState)

	python3.PyEval_RestoreThread(mainState)
	si, err := python3.NewSubInterpreter(python3.DefaultSubInterpreterConfig())
	if err != nil {
		t.Fatalf("NewSubInterpreter: %v", err)
	}
	defer si.Close()
	si.Enter()
	defer si.Leave()

	if err := python3.LoadSource(`x = 42`); err != nil {
		t.Fatalf("LoadSource: %v", err)
	}

	// "x" exists but is not callable.
	if _, err := python3.CallJSONFunctionByName("x", `null`); !errors.Is(err, python3.ErrUnknownFunction) {
		t.Fatalf("expected ErrUnknownFunction for non-callable, got %v", err)
	}
	// "missing" does not exist at all.
	if _, err := python3.CallJSONFunctionByName("missing", `null`); !errors.Is(err, python3.ErrUnknownFunction) {
		t.Fatalf("expected ErrUnknownFunction for missing name, got %v", err)
	}
}

// TestCallJSONFunctionByName_PyException verifies a Python-side
// exception is surfaced as a non-nil error with a useful message.
func TestCallJSONFunctionByName_PyException(t *testing.T) {
	python3.Py_InitializeEx(false)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	mainState := python3.PyEval_SaveThread()
	defer python3.PyEval_RestoreThread(mainState)

	python3.PyEval_RestoreThread(mainState)
	si, err := python3.NewSubInterpreter(python3.DefaultSubInterpreterConfig())
	if err != nil {
		t.Fatalf("NewSubInterpreter: %v", err)
	}
	defer si.Close()
	si.Enter()
	defer si.Leave()

	if err := python3.LoadSource(`
def boom(_):
    raise ValueError("kaboom")
`); err != nil {
		t.Fatalf("LoadSource: %v", err)
	}

	_, err = python3.CallJSONFunctionByName("boom", `null`)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "kaboom") {
		t.Fatalf("expected error to mention 'kaboom', got %v", err)
	}
}

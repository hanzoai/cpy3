/*
SPDX-License-Identifier: MIT
Hanzo additions, 2026-05-19.

These tests deliberately run in package python3 (not python3_test)
so they share Py_Initialize state with the other in-package tests
that touch global Python state. The harness pattern matches
subinterp_test.go: pin OS thread, Py_Initialize (idempotent), create
sub-interpreter, run, tear down.
*/

package python3

import (
	"errors"
	gort "runtime"
	"strings"
	"testing"
)

// initAndPin pins the goroutine to its OS thread and brings the main
// interpreter up to a state where NewSubInterpreter can be called
// (caller thread holds the main GIL). Caller must Defer the returned
// teardown which unpins. The teardown does NOT touch Python state —
// by the time the test body runs `defer si.Close()`, the sub-interp's
// state has been torn down and the calling thread holds no GIL.
//
// Subsequent tests can call this safely; Py_Initialize is idempotent.
func initAndPin(t *testing.T) (teardown func()) {
	t.Helper()
	gort.LockOSThread()
	Py_Initialize()
	// Py_Initialize leaves the calling thread holding the main GIL.
	// Caller must hold it before NewSubInterpreter — we're already
	// there.
	return func() {
		// Sub-interp Close already released the GIL. We do NOT call
		// PyEval_SaveThread here — that would fail without a thread
		// state. Just unpin.
		gort.UnlockOSThread()
	}
}

// TestCallJSONFunctionByName_HappyPath exercises the consolidated
// JSON hot path. It loads a tiny Python source into a fresh
// sub-interpreter, calls it, and asserts the JSON round-trip works.
func TestCallJSONFunctionByName_HappyPath(t *testing.T) {
	defer initAndPin(t)()

	si, err := NewSubInterpreter(DefaultSubInterpreterConfig())
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
	if err := LoadSource(src); err != nil {
		t.Fatalf("LoadSource: %v", err)
	}

	out, err := CallJSONFunctionByName("echo", `{"a":1,"b":"two"}`)
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
	defer initAndPin(t)()

	si, err := NewSubInterpreter(DefaultSubInterpreterConfig())
	if err != nil {
		t.Fatalf("NewSubInterpreter: %v", err)
	}
	defer si.Close()

	si.Enter()
	defer si.Leave()

	if err := LoadSource(`x = 42`); err != nil {
		t.Fatalf("LoadSource: %v", err)
	}

	// "x" exists but is not callable.
	if _, err := CallJSONFunctionByName("x", `null`); !errors.Is(err, ErrUnknownFunction) {
		t.Fatalf("expected ErrUnknownFunction for non-callable, got %v", err)
	}
	// "missing" does not exist at all.
	if _, err := CallJSONFunctionByName("missing", `null`); !errors.Is(err, ErrUnknownFunction) {
		t.Fatalf("expected ErrUnknownFunction for missing name, got %v", err)
	}
}

// TestCallJSONFunctionByName_PyException verifies a Python-side
// exception is surfaced as a non-nil error with a useful message.
func TestCallJSONFunctionByName_PyException(t *testing.T) {
	defer initAndPin(t)()

	si, err := NewSubInterpreter(DefaultSubInterpreterConfig())
	if err != nil {
		t.Fatalf("NewSubInterpreter: %v", err)
	}
	defer si.Close()

	si.Enter()
	defer si.Leave()

	if err := LoadSource(`
def boom(_):
    raise ValueError("kaboom")
`); err != nil {
		t.Fatalf("LoadSource: %v", err)
	}

	_, err = CallJSONFunctionByName("boom", `null`)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "kaboom") {
		t.Fatalf("expected error to mention 'kaboom', got %v", err)
	}
}

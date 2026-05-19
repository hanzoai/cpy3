/*
SPDX-License-Identifier: MIT
Hanzo additions, 2026-05-19.

Sub-interpreter wrapper tests.
*/

package python3

import (
	gort "runtime"
	"testing"
)

func TestIsGILDisabled_RuntimeQuery(t *testing.T) {
	// Just make sure it doesn't blow up. Value depends on whether
	// the linked CPython was built with --disable-gil. We log it so
	// CI logs show which build we're on.
	v := IsGILDisabled()
	t.Logf("IsGILDisabled = %v (Python version = %s)", v, Py_GetVersion())
}

// TestSubInterpreter_CreateAndClose verifies the smallest possible
// PEP 684 round trip — create a sub-interpreter with default config,
// enter it, run a trivial Python statement, leave, close.
//
// Requires Py_Initialize() to have run AND the calling thread to hold
// the main interpreter's GIL when we call NewSubInterpreter — the
// test harness handles that by pinning + initialising at the top.
func TestSubInterpreter_CreateAndClose(t *testing.T) {
	gort.LockOSThread()
	defer gort.UnlockOSThread()

	Py_Initialize()
	// Caller invariant: hold the main GIL before NewSubInterpreter.
	// Py_Initialize leaves the calling thread holding the GIL.

	si, err := NewSubInterpreter(DefaultSubInterpreterConfig())
	if err != nil {
		t.Fatalf("NewSubInterpreter: %v", err)
	}
	defer si.Close()

	if si.ThreadState() == nil {
		t.Fatal("ThreadState returned nil after successful create")
	}

	si.Enter()
	// Trivial: run "1+1" as a Python expression via PyRun_SimpleString.
	rc := PyRun_SimpleString("x = 1 + 1")
	si.Leave()
	if rc != 0 {
		t.Fatalf("PyRun_SimpleString returned %d, want 0", rc)
	}
}

/*
SPDX-License-Identifier: MIT
Hanzo additions, 2026-05-19.

Sub-interpreter wrapper (PEP 684 — OWN_GIL sub-interpreters). The
recommended primitive on 3.12+ for tenant / per-worker isolation.

Use one SubInterpreter per concurrent worker. Each has its own GIL on
3.12+, so two SubInterpreters from two goroutines truly run in parallel
on two OS threads.

API discipline:
  - Each call into SubInterpreter MUST be made from a goroutine pinned
    to its OS thread (runtime.LockOSThread). The wrapper pins on Enter
    and unpins on Leave; do not Enter again on a different OS thread
    between paired Enter/Leave calls.
  - Free-threaded builds (PEP 703, --disable-gil) skip the GIL ceremony
    silently; the wrapper still works because PyGILState_Ensure is a
    no-op there.
*/

package python3

/*
#include "Python.h"
#include "macro.h"
#include <stdlib.h>
#include <string.h>

// Compat: PyInterpreterConfig + Py_NewInterpreterFromConfig only exist on 3.12+.
#if PY_VERSION_HEX >= 0x030C0000
static PyThreadState *_hz_new_subinterp(int allow_threads,
                                        int allow_daemon_threads,
                                        int check_multi_interp_extensions,
                                        int own_gil,
                                        char **err) {
    PyInterpreterConfig cfg = {
        .use_main_obmalloc = 0,
        .allow_fork = 0,
        .allow_exec = 0,
        .allow_threads = allow_threads,
        .allow_daemon_threads = allow_daemon_threads,
        .check_multi_interp_extensions = check_multi_interp_extensions,
        .gil = own_gil ? PyInterpreterConfig_OWN_GIL : PyInterpreterConfig_SHARED_GIL,
    };
    PyThreadState *ts = NULL;
    PyStatus st = Py_NewInterpreterFromConfig(&ts, &cfg);
    if (PyStatus_Exception(st)) {
        const char *m = st.err_msg ? st.err_msg : "Py_NewInterpreterFromConfig failed";
        *err = strdup(m);
        return NULL;
    }
    return ts;
}
#else
static PyThreadState *_hz_new_subinterp(int allow_threads,
                                        int allow_daemon_threads,
                                        int check_multi_interp_extensions,
                                        int own_gil,
                                        char **err) {
    (void)allow_threads; (void)allow_daemon_threads;
    (void)check_multi_interp_extensions; (void)own_gil;
    // Fall back to the legacy SHARED_GIL Py_NewInterpreter on 3.11 and older.
    PyThreadState *ts = Py_NewInterpreter();
    if (ts == NULL) {
        *err = strdup("Py_NewInterpreter returned NULL");
    }
    return ts;
}
#endif
*/
import "C"

import (
	"errors"
	gort "runtime"
	"unsafe"
)

// SubInterpreterConfig configures a new sub-interpreter.
type SubInterpreterConfig struct {
	// AllowThreads — Python threads enabled inside this interpreter.
	// Default: true.
	AllowThreads bool

	// AllowDaemonThreads — daemon threads (created but never joined)
	// allowed. Default: false. PEP 684 recommends false.
	AllowDaemonThreads bool

	// CheckMultiInterpExtensions — refuse C extensions that don't
	// declare multi-interpreter support. Default: true. PEP 684
	// recommends true.
	CheckMultiInterpExtensions bool

	// OwnGIL — give this sub-interpreter its own GIL (PEP 684 default
	// on 3.12+). Required for true parallelism with another interpreter.
	// On 3.11 and older this flag is ignored (only SHARED_GIL exists).
	// Default: true.
	OwnGIL bool
}

// DefaultSubInterpreterConfig returns the recommended PEP 684 defaults:
// own GIL, threads on, no daemon threads, refuse non-isolated C exts.
func DefaultSubInterpreterConfig() SubInterpreterConfig {
	return SubInterpreterConfig{
		AllowThreads:               true,
		AllowDaemonThreads:         false,
		CheckMultiInterpExtensions: true,
		OwnGIL:                     true,
	}
}

// SubInterpreter is one CPython sub-interpreter.
//
// Lifecycle:
//
//	si, err := NewSubInterpreter(DefaultSubInterpreterConfig())
//	defer si.Close()
//	si.Enter()
//	// ... call any python3.* function — they target si now
//	si.Leave()
//
// CPython requires that NewSubInterpreter is called on a thread that
// holds the main interpreter's GIL. The recommended pattern is:
//
//	Py_Initialize()
//	// ... do main interpreter setup ...
//	mainState := PyEval_SaveThread()   // release main GIL
//	gort.LockOSThread()
//	// reacquire main GIL on this OS thread before NewSubInterpreter:
//	PyEval_RestoreThread(mainState)
//	si, err := NewSubInterpreter(DefaultSubInterpreterConfig())
//
// The wrapper handles the GIL save/restore inside NewSubInterpreter
// itself: it expects the caller to hold the main GIL before the call
// and returns with the caller's OS thread no longer holding any GIL.
type SubInterpreter struct {
	ts *C.PyThreadState
}

// NewSubInterpreter creates a sub-interpreter and returns it. Caller
// must hold the main interpreter's GIL before calling; the call returns
// with no GIL held on the calling OS thread.
//
// Returns an error if the sub-interpreter could not be created. The
// most common failure on 3.12+ is "interpreter creation failed" when
// the caller is not pinned to an OS thread or did not actually hold
// the main GIL.
func NewSubInterpreter(cfg SubInterpreterConfig) (*SubInterpreter, error) {
	var cErr *C.char
	ts := C._hz_new_subinterp(
		boolC(cfg.AllowThreads),
		boolC(cfg.AllowDaemonThreads),
		boolC(cfg.CheckMultiInterpExtensions),
		boolC(cfg.OwnGIL),
		&cErr,
	)
	if ts == nil {
		msg := "unknown error"
		if cErr != nil {
			msg = C.GoString(cErr)
			C.free(unsafe.Pointer(cErr))
		}
		return nil, errors.New("cpy3: NewSubInterpreter: " + msg)
	}
	// Release the new sub-interp's GIL — caller must Enter to use it.
	C.PyEval_SaveThread()
	return &SubInterpreter{ts: ts}, nil
}

// Enter acquires this sub-interpreter's GIL on the calling OS thread.
// Caller MUST have called runtime.LockOSThread() first. Pair with Leave.
func (s *SubInterpreter) Enter() {
	gort.LockOSThread()
	C.PyEval_RestoreThread(s.ts)
}

// Leave releases this sub-interpreter's GIL and unpins the OS thread.
func (s *SubInterpreter) Leave() {
	C.PyEval_SaveThread()
	gort.UnlockOSThread()
}

// ThreadState returns the underlying PyThreadState pointer. For
// interop with code that needs the raw value. Most callers should not
// need this.
func (s *SubInterpreter) ThreadState() *PyThreadState {
	return (*PyThreadState)(s.ts)
}

// Close tears down the sub-interpreter. Caller must NOT hold any GIL;
// Close acquires the target sub-interp's GIL itself.
//
// After Close, the SubInterpreter is unusable and further Enter/Leave
// calls will crash. Idempotent — second Close is a no-op.
func (s *SubInterpreter) Close() error {
	if s == nil || s.ts == nil {
		return nil
	}
	gort.LockOSThread()
	defer gort.UnlockOSThread()
	C.PyEval_RestoreThread(s.ts)
	C.Py_EndInterpreter(s.ts)
	s.ts = nil
	// We don't swap back to a known main state — the caller must
	// PyEval_RestoreThread their own main state on this OS thread if
	// they need it. Most callers spawn a goroutine per sub-interp and
	// let it die.
	return nil
}

// IsGILDisabled reports whether the linked CPython was built with the
// free-threading (--disable-gil, PEP 703) flag. On such builds, the
// GIL functions in this package become no-ops but remain safe to call;
// OWN_GIL sub-interpreters are still useful for state isolation but
// no longer required for parallelism.
//
// Returns false on Python 3.12 and older (free-threading is 3.13+).
func IsGILDisabled() bool {
	return C._go_hz_gil_disabled() != 0
}

func boolC(b bool) C.int {
	if b {
		return 1
	}
	return 0
}

/*
Unless explicitly stated otherwise all files in this repository are licensed
under the MIT License.
This product includes software developed at Datadog (https://www.datadoghq.com/).
Copyright 2018 Datadog, Inc.
*/

package python3

/*
#include "Python.h"
#include "macro.h"
*/
import "C"

// PyEval_InitThreads / PyEval_ReInitThreads were no-ops since Python 3.9
// and removed in Python 3.13. They are kept here for API compatibility;
// on 3.13+ they polyfill to no-ops via macro.h.
//
// PyGILState_Ensure / PyGILState_Release continue to work but are
// essentially no-ops on free-threaded (--disable-gil, PEP 703) builds.
// Use IsGILDisabled() to detect.

//PyThreadState : https://docs.python.org/3/c-api/init.html#c.PyThreadState
type PyThreadState C.PyThreadState

//PyGILState is an opaque “handle” to the thread state when PyGILState_Ensure() was called, and must be passed to PyGILState_Release() to ensure Python is left in the same state
type PyGILState C.PyGILState_STATE

//PyEval_InitThreads : https://docs.python.org/3/c-api/init.html#c.PyEval_InitThreads
func PyEval_InitThreads() {
	C.PyEval_InitThreads()
}

//PyEval_ThreadsInitialized : https://docs.python.org/3/c-api/init.html#c.PyEval_ThreadsInitialized
func PyEval_ThreadsInitialized() bool {
	return C.PyEval_ThreadsInitialized() != 0
}

//PyEval_SaveThread : https://docs.python.org/3/c-api/init.html#c.PyEval_SaveThread
func PyEval_SaveThread() *PyThreadState {
	return (*PyThreadState)(C.PyEval_SaveThread())
}

//PyEval_RestoreThread : https://docs.python.org/3/c-api/init.html#c.PyEval_RestoreThread
func PyEval_RestoreThread(tstate *PyThreadState) {
	C.PyEval_RestoreThread((*C.PyThreadState)(tstate))
}

//PyThreadState_Get : https://docs.python.org/3/c-api/init.html#c.PyThreadState_Get
func PyThreadState_Get() *PyThreadState {
	return (*PyThreadState)(C.PyThreadState_Get())
}

//PyThreadState_Swap : https://docs.python.org/3/c-api/init.html#c.PyThreadState_Swap
func PyThreadState_Swap(tstate *PyThreadState) *PyThreadState {
	return (*PyThreadState)(C.PyThreadState_Swap((*C.PyThreadState)(tstate)))
}

//PyEval_ReInitThreads : https://docs.python.org/3/c-api/init.html#c.PyEval_ReInitThreads
func PyEval_ReInitThreads() {
	C.PyEval_ReInitThreads()
}

//PyGILState_Ensure : https://docs.python.org/3/c-api/init.html#c.PyGILState_Ensure
func PyGILState_Ensure() PyGILState {
	return PyGILState(C.PyGILState_Ensure())
}

//PyGILState_Release : https://docs.python.org/3/c-api/init.html#c.PyGILState_Release
func PyGILState_Release(state PyGILState) {
	C.PyGILState_Release(C.PyGILState_STATE(state))
}

//PyGILState_GetThisThreadState : https://docs.python.org/3/c-api/init.html#c.PyGILState_GetThisThreadState
func PyGILState_GetThisThreadState() *PyThreadState {
	return (*PyThreadState)(C.PyGILState_GetThisThreadState())
}

//PyGILState_Check : https://docs.python.org/3/c-api/init.html#c.PyGILState_Check
func PyGILState_Check() bool {
	return C.PyGILState_Check() == 1
}

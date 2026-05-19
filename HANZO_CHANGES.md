# Hanzo fork changes

This fork (`github.com/hanzoai/cpy3`) is based on
[`github.com/go-python/cpy3`](https://github.com/go-python/cpy3) `v0.2.0`
(commit `98d3175`). The upstream go-python team's work is the foundation —
all credit for the original Go bindings goes to them. License remains
MIT (see `LICENSE`).

## What changed

### Python 3.12+ removed-API compat (Phase 2)

Python 3.12 dropped the per-type freelist clear functions and 3.13 dropped
several legacy lifecycle / sys helpers. We polyfill these as no-ops or as
thin `PyConfig`-based shims via `macro.h` so existing call sites keep
compiling. New code should prefer the documented `PyConfig` replacements
directly.

Removed on 3.12 (polyfilled to return `0`):

  - `PyFloat_ClearFreeList`
  - `PyDict_ClearFreeList`
  - `PyList_ClearFreeList`
  - `PyTuple_ClearFreeList`
  - `PySet_ClearFreeList`
  - `PyMethod_ClearFreeList`
  - `PyCFunction_ClearFreeList`
  - `PyFrame_ClearFreeList`
  - `PyAsyncGen_ClearFreeList`

These were "best-effort free some cached memory" helpers; the cyclic GC
handles freelists now and the calls were no-ops in release-mode builds
even before 3.12.

Removed on 3.13 (polyfilled as no-ops or PyConfig shims):

  - `PyEval_InitThreads`        → no-op (threads always init since 3.7)
  - `PyEval_ReInitThreads`      → no-op
  - `PyEval_AcquireLock`        → no-op
  - `PyEval_ReleaseLock`        → no-op
  - `PyEval_ThreadsInitialized` → returns `1`
  - `Py_SetProgramName`         → `PyConfig_SetString(program_name)`
  - `Py_SetPath`                → `PyConfig_SetString(pythonpath_env)`
  - `Py_SetPythonHome`          → `PyConfig_SetString(home)`
  - `Py_SetStandardStreamEncoding` → `PyConfig_SetBytesString(stdio_*)`
  - `PySys_SetArgv` / `PySys_SetArgvEx` → `PyConfig_SetArgv`
  - `PySys_AddWarnOption`       → `PyWideStringList_Append(warnoptions)`
  - `PySys_AddXOption`          → `PyWideStringList_Append(xoptions)`
  - `PySys_SetPath`             → `PyConfig_SetString(pythonpath_env)`

NOTE: the polyfilled lifecycle / sys helpers only take effect when called
*before* `Py_Initialize`. Calling them after init is a no-op (matches the
historical behaviour for some of these). Convert call sites to the
`PyConfig` API for full fidelity.

### pkg-config: `python3-embed` (was `python3`)

The cgo `pkg-config:` directive now requests `python3-embed`, which links
`libpython` for embedding. The old `python3` name only provided headers
and broke linking on modern distros where the split was made canonical.

### Sub-interpreter wrapper (PEP 684) — additive API

`subinterp.go` adds a Go-friendly wrapper around `Py_NewInterpreterFromConfig`:

```go
si, err := python3.NewSubInterpreter(python3.DefaultSubInterpreterConfig())
defer si.Close()
si.Enter()
defer si.Leave()
// ... call any python3.* function — runs in the sub-interp ...
```

Defaults follow PEP 684 recommendations: own GIL, threads on, no daemon
threads, refuse non-isolated C extensions. On Python 3.11 and older the
config is ignored and the wrapper falls back to `Py_NewInterpreter`
(shared GIL only). Use one sub-interpreter per concurrent tenant /
worker for state + crash isolation.

### Free-threading awareness (PEP 703) — additive

`IsGILDisabled() bool` reports whether the linked CPython was built with
`--disable-gil` (3.13+ `python3.13t`). The classical GIL wrappers
(`PyGILState_Ensure`, etc.) remain callable on free-threaded builds —
they become no-ops, harmless. Use the boolean to decide whether the
pool-of-sub-interpreters trick is needed for parallelism (it isn't, on
free-threaded builds).

### Module path: `github.com/hanzoai/cpy3`

Go module is now `github.com/hanzoai/cpy3`. Go `1.21+`. Package name
unchanged — still `python3`.

## What did not change

  - License is still MIT (see `LICENSE`).
  - Upstream attribution is preserved in every file header.
  - Package name (`python3`) is preserved so adoption is a one-line
    import swap from `"github.com/go-python/cpy3"` to
    `"github.com/hanzoai/cpy3"`.

## Known limitations / open items

The upstream test suite has a handful of cases that crash on Python 3.13
for reasons unrelated to the removed-API fixes:

  - `TestErrorSetObject` / `TestErrorSetNone` / `TestErrorSetObjectEx` —
    invoke `PyErr_PrintEx` against `sys.stderr` of an interpreter that
    was initialised without a real stderr. Behaviour changed in 3.13.
  - `TestErrorFetchRestore` / `TestErrorGetSetExcInfo` — rely on the
    pre-3.12 exception fetch/restore semantics that PEP 657 reshaped.
  - `TestImportModuleEx` — crashes inside `PyImport_ImportModuleEx`
    when `globals` is `nil`; a 3.13 contract change.

CI (`.github/workflows/test.yml`) runs a curated subset that passes on
3.11 / 3.12 / 3.13. Fixing the upstream tests is tracked separately — it
is migration work to the post-3.12 exception API, not removed-symbol
work.

## Why fork

The upstream go-python/cpy3 has not had a release tagged since 0.2.0
(Aug 2022). It does not compile against Python 3.12+ out of the box.
DataDog/go-python3 (the parent of go-python/cpy3) was archived. We need
a working binding for hanzo/base's pyvm extension runtime, with current
Python and sub-interpreter support. Forking and pinning was less code
than maintaining a long-lived patch series against upstream.

## Credits

Original work © 2018 Datadog, Inc. and the go-python authors. Released
under MIT (see `LICENSE`). Hanzo additions © 2026 Hanzo, Inc., released
under the same MIT terms.

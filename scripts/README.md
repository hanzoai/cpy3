# cpy3 scripts

## build-python313t.sh

Builds a local CPython 3.13.x with `--disable-gil` (PEP 703 free-
threading). Output lives at `./python313t-build/` relative to the repo
root. Idempotent — re-running is a no-op if the binary is present and
correctly reports `sys._is_gil_enabled() == False`.

### Prereqs

macOS:

    xcode-select --install

Debian/Ubuntu:

    sudo apt-get install -y build-essential libssl-dev libffi-dev \
        zlib1g-dev libbz2-dev liblzma-dev libsqlite3-dev libreadline-dev

### Run

    ./scripts/build-python313t.sh

Override patch version (defaults to `3.13.5`):

    PYTHON_VERSION=3.13.7 ./scripts/build-python313t.sh

### Use the built Python with cpy3

cpy3 links against `python3-embed` via `pkg-config`. The build script
writes a `python3-embed.pc` pointing at the new install so callers just
need to prepend its directory:

    export PKG_CONFIG_PATH="$PWD/python313t-build/lib/pkgconfig:${PKG_CONFIG_PATH:-}"
    # macOS:
    export DYLD_LIBRARY_PATH="$PWD/python313t-build/lib:${DYLD_LIBRARY_PATH:-}"
    # Linux:
    export LD_LIBRARY_PATH="$PWD/python313t-build/lib:${LD_LIBRARY_PATH:-}"
    go test ./...

`cpy3.IsGILDisabled()` will now return `true`.

### Time budget

Clean build: 10-20 minutes on a 2024-era laptop with `-j$(nproc)`.
Incremental rebuilds (rare — the tarball doesn't change) are minutes.

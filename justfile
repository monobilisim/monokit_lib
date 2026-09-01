# justfile for monokit_lib — build, vet & test recipes.
# Run `just` or `just --list` to see the available recipes.

version := env("VERSION", "devel")
image   := "monokit-lib-tests"
bindir  := justfile_directory() / "bin"

# Show the available recipes.
default:
    @just --list

# Type-check the library and its tests for the host platform.
build:
    @echo "Building monokit_lib {{version}} for the host platform"
    go build ./...

# Build the smoke binary that links the library, for every release target.
# This is the only consumer of cmd/monokit_lib: it exists so cross-compilation
# is actually verified, including the shutdown_notifier build-tag split.
build-all: (build-target "linux" "amd64") (build-target "linux" "arm64") (build-target "windows" "amd64") (build-target "freebsd" "amd64")

# Cross-compile the smoke binary for a single GOOS/GOARCH target.
# CGO is off here: the sqlite driver needs cgo, but a cross-build only has to
# prove the non-cgo code paths and build tags still compile.
build-target goos goarch:
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p "{{bindir}}"
    ext=""
    [ "{{goos}}" = "windows" ] && ext=".exe"
    out="{{bindir}}/monokit_lib_{{version}}_{{goos}}_{{goarch}}${ext}"
    echo "Building monokit_lib {{version}} for {{goos}} {{goarch}}"
    CGO_ENABLED=0 GOOS={{goos}} GOARCH={{goarch}} go build -ldflags "-X 'main.version={{version}}'" -o "$out" ./cmd/monokit_lib

# go vet over the library and its tests.
vet:
    go vet ./...

# Fail if anything is not gofmt-clean.
fmt-check:
    #!/usr/bin/env bash
    set -euo pipefail
    unformatted="$(gofmt -l .)"
    if [ -n "$unformatted" ]; then
        echo "Not gofmt-clean:"
        echo "$unformatted"
        exit 1
    fi
    echo "gofmt clean"

# Run the tests inside a Podman container.
# `just test` runs every test; `just test TestFoo` runs only matching tests.
# Tests ALWAYS run inside Podman — never directly on the host, because they
# write to /etc/mono, /var/log and /var/lib.
test filter="":
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p "{{justfile_directory()}}/logs"
    podman build -t {{image}} -f Containerfile .
    podman run --rm -t \
        -v {{image}}-go-mod-cache:/go/pkg/mod \
        -v {{image}}-go-build-cache:/root/.cache/go-build \
        -v "{{justfile_directory()}}/logs":/artifacts \
        -e TEST_RUN="{{filter}}" \
        -e HOST_UID="$(id -u)" \
        -e HOST_GID="$(id -g)" \
        {{image}}

# Everything CI runs, in CI's order.
check: fmt-check vet build test

# Remove build output and the podman image.
clean:
    rm -rf "{{bindir}}" "{{justfile_directory()}}/logs"
    -podman rmi {{image}}

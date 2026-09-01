# Containerfile for running monokit_lib's tests under Podman.
#
# The tests are mock tests against a real sqlite file. Nothing here needs
# systemd or a service fixture — the only reason to containerise is that
# InitConfig, InitLogger and InitializeDatabase write to /etc/mono, /var/log
# and /var/lib, so the suite needs a throwaway root it is allowed to scribble on.
#
# The golang image already ships gcc, which is all the cgo sqlite driver needs,
# so there is no apt-get step. Invoke via `just test` (or `just test TestName`).
FROM golang:1.25-trixie

ENV CGO_ENABLED=1
# Makes IsTestMode() true, so the alarm senders take the no-network path.
ENV TEST=true

WORKDIR /app

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

# InitConfig requires /etc/mono/global.yml and nothing else. Tests that need a
# plugin config file write it themselves and clean up after.
RUN mkdir -p /etc/mono && cp config/global.yml /etc/mono/global.yml

# go test's exit code becomes the container's exit code.
#
# TEST_RUN is an optional -run filter from `just test <filter>`; it defaults to
# "." which matches every test, so the flag is always present and always a
# single properly-quoted word. Building the flag conditionally would not work:
# quote removal happens before parameter expansion, so the quotes inside a
# ${TEST_RUN:+-run "$TEST_RUN"} would reach go test literally.
COPY <<'EOF' /run-tests.sh
#!/bin/bash
# bash, not sh: the transcript is teed so it streams live, and only bash has
# pipefail — under dash $? would be tee's status and a failing test would exit 0.
set -o pipefail

# The transcript goes to its own file rather than to global.yml's log-location.
# The plugin harness tees into /var/log/monokit2.log, which is also what the
# library's own logger has open in append mode — two writers at two offsets in
# one file clobber each other's regions. Keeping them apart means monokit2.log
# is purely what the library logged.
go test -v -count=1 -coverprofile=/tmp/coverage.out -run "${TEST_RUN:-.}" ./... 2>&1 \
    | tee /tmp/test-output.log
code=$?

if [ -d /artifacts ]; then
    # Same three artifacts the plugin suites produce, from the paths
    # config/global.yml points the library at.
    cp /var/lib/monokit2/monokit2.db /artifacts/monokit2.db 2>/dev/null
    cp /var/log/monokit2.log /artifacts/monokit2.log 2>/dev/null
    cp /tmp/test-output.log /artifacts/test-output.log 2>/dev/null

    cp /tmp/coverage.out /artifacts/coverage.out 2>/dev/null
    go tool cover -func=/tmp/coverage.out > /artifacts/coverage.txt 2>/dev/null

    if [ -n "$HOST_UID" ]; then
        chown "$HOST_UID:${HOST_GID:-$HOST_UID}" /artifacts/* 2>/dev/null
    fi
fi

exit $code
EOF

RUN chmod +x /run-tests.sh

ENTRYPOINT ["/run-tests.sh"]

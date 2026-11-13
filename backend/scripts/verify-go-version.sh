#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
expected_go="$(awk '$1 == "go" { print $2; exit }' "$repository_root/backend/go.mod")"
test -n "$expected_go"
actual_go="$(go env GOVERSION)"
test "$actual_go" = "go${expected_go}"
docker_go="$(sed -n 's/^ARG GOLANG_IMAGE=golang:\(.*\)-alpine$/\1/p' "$repository_root/Dockerfile")"
test "$docker_go" = "$expected_go"
printf 'Go toolchain matches backend/go.mod: %s\n' "$actual_go"

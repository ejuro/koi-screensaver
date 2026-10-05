#!/bin/bash

# Build the koi pond for both architectures into bin/, the binaries the
# plugin ships so nobody installing it needs Go.
set -euo pipefail
cd "$(dirname "$0")/src"
for arch in amd64:x86_64 arm64:aarch64; do
  CGO_ENABLED=0 GOOS=linux GOARCH=${arch%%:*} go build -trimpath -ldflags "-s -w" -o "../bin/koi-pond-${arch##*:}" .
done
ls -l ../bin

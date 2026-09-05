#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export PATH="$(go env GOPATH)/bin:$PATH"
if ! command -v protoc >/dev/null; then
  echo "protoc not found" >&2
  exit 1
fi
if ! command -v protoc-gen-go >/dev/null; then
  echo "installing protoc-gen-go@v1.28.1"
  go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28.1
fi
mkdir -p internal/pb
protoc -I api/proto --go_out=internal/pb --go_opt=paths=source_relative api/proto/gbmj.proto
echo "generated internal/pb/gbmj.pb.go"

#!/usr/bin/env bash
# 隔离测试：不接触 VMISS/3X-UI。在临时目录跑二进制。
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export PATH="${HOME}/opt/go/bin:$PATH"
cd "$ROOT"
go test ./...
mkdir -p /tmp/x-ui-mini-dist
CGO_ENABLED=0 go build -trimpath -ldflags '-s -w -X github.com/xinruiown/x-ui-mini/internal/version.Version=0.1.0' -o /tmp/x-ui-mini-dist/x-ui-mini ./cmd/x-ui-mini
DATA="$(mktemp -d)"
export XUIMINI_DATA="$DATA"
/tmp/x-ui-mini-dist/x-ui-mini version
/tmp/x-ui-mini-dist/x-ui-mini status
BK="$DATA/bak.tgz"
/tmp/x-ui-mini-dist/x-ui-mini backup "$BK"
test -s "$BK"
/tmp/x-ui-mini-dist/x-ui-mini restore "$BK"
echo "unit+backup OK data=$DATA"

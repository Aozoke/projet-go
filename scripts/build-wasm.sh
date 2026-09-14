#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PUBLIC_DIR="$ROOT_DIR/web/public"

mkdir -p "$PUBLIC_DIR"
cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" "$PUBLIC_DIR/wasm_exec.js"

GOOS=js GOARCH=wasm go build -o "$PUBLIC_DIR/wasmredis.wasm" "$ROOT_DIR/cmd/wasm"

echo "WASM build ok: web/public/wasmredis.wasm"

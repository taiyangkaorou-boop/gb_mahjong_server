#!/usr/bin/env bash
# 可选：单独编译动态库。默认 CGO 会把 C++ 源编进 server。
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SRC="$ROOT/third_party/GB-Mahjong"
OUT="$ROOT/cgo/gbmj"
mkdir -p "$OUT/build"
g++ -std=c++11 -O3 -fPIC -shared \
  -I "$SRC/mahjong" -I "$SRC/console" \
  "$ROOT/cgo/gbmj/cgbmj.cpp" \
  "$SRC/mahjong/tile.cpp" \
  "$SRC/mahjong/pack.cpp" \
  "$SRC/mahjong/handtiles.cpp" \
  "$SRC/mahjong/fan.cpp" \
  "$SRC/console/print.cpp" \
  "$SRC/console/console.cpp" \
  -o "$OUT/libgbmj.so"
echo "built $OUT/libgbmj.so"

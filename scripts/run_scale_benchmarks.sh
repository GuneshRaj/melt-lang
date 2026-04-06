#!/bin/zsh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

env GOCACHE="$ROOT/build/.gocache" go run ./scripts/gen_prices_100k.go
env GOCACHE="$ROOT/build/.gocache" go run ./compiler/cmd/meltc build examples/cpu_scale.melt -o build/cpu_scale
env GOCACHE="$ROOT/build/.gocache" go run ./compiler/cmd/meltc build examples/gpu_scale.melt -o build/gpu_scale

echo "CPU:"
time ./build/cpu_scale >/tmp/melt_cpu_scale.log

echo "GPU:"
time ./build/gpu_scale >/tmp/melt_gpu_scale.log

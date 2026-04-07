# Melt Lang

Melt is a small statically typed data-parallel programming language aimed at Apple Silicon workflows.

The compiler is written in Go and currently supports:

- parsing and semantic checking of `.melt` files
- MIR lowering
- MIR interpreter execution
- early Swift/Metal native build generation

## Status

This project is in active development.

The stable path today is:

- `meltc check`
- `meltc ast`
- `meltc mir`
- `meltc run`
- `meltc build` for the currently supported scale examples

The native `meltc build` path works for the current narrow benchmark slice, but the backend is still early and intentionally limited.

## Build The Compiler

From the repository root:

```bash
mkdir -p bin
env GOCACHE="$PWD/build/.gocache" go build -o ./bin/meltc ./compiler/cmd/meltc
```

You can then run:

```bash
./bin/meltc check examples/cpu_scale_f32.melt
```

## `meltc` Usage

```bash
meltc check <file.melt>
meltc ast <file.melt>
meltc mir <file.melt>
meltc run <file.melt>
meltc build <file.melt> -o <output>
```

### Commands

`check`

- lexes, parses, and semantically validates the source file

`ast`

- prints the parsed AST as JSON

`mir`

- prints the lowered MIR as JSON

`run`

- lexes, parses, checks, lowers to MIR, and runs the program with the Go interpreter
- this is the main working execution mode right now

`build`

- generates Swift and Metal output and builds a native executable for the currently supported subset

## Language Specification

This section documents the Melt subset that is implemented today.

### Design Model

Melt currently has two execution domains:

- host functions: regular `fn`
- kernel functions: `kernel fn`

The intended usage model is:

- host code performs orchestration, file IO, CSV loading, printing, and saving
- kernel code performs data-parallel array computation

For the current implemented slice, both host and kernel functions can express the same simple scale operation, and the compiler chooses the CPU or GPU path based on whether `kernel` is present.

### Source Files

- Melt source files use the `.melt` extension
- the compiler entrypoint is `meltc`

### Keywords

The current Melt keyword set used by the implemented subset is:

- `struct`
- `fn`
- `kernel`
- `return`
- `if`
- `elif`
- `else`
- `and`
- `or`
- `not`
- `true`
- `false`

Notes:

- `if`, `elif`, and `else` are part of the language grammar, although the main benchmark/examples path is currently focused on straight-line programs
- `true` and `false` are boolean literals
- `kernel fn` is a two-keyword form that marks a function for the GPU-oriented kernel path

### Indentation And Layout

Melt uses indentation-sensitive syntax.

Rules:

- blocks begin after a trailing `:`
- nested statements are indented on following lines
- spaces are allowed for indentation
- tabs are rejected
- indentation width does not have to be fixed globally
- indentation must still be structurally consistent by block

Example:

```melt
fn main():
    rows = csv.load("data/prices_100k.csv", as=Array[PriceRow])
    print(rows)
```

### Comments

Melt implements C-style comments in source code:

- line comments: `// ...`
- block comments: `/* ... */`

Supported today:

- full-line comments
- inline comments after code
- multi-line block comments
- comment-only lines inside indented blocks

Current limitation:

- block comments do not nest

Examples:

```melt
// Load the CSV rows
rows = csv.load("data/prices_100k.csv", as=Array[PriceRow])

/* Scale the close column
   with a constant factor. */
out = scale(close, 1.1)
```

### Declarations

#### Structs

Structs define named record types with fixed fields.

Syntax:

```melt
struct PriceRow:
    ts: Int64
    close: Float32
```

Rules:

- struct names must be unique
- field names inside a struct must be unique
- field types must be valid Melt types

#### Functions

Host function syntax:

```melt
fn scale(x: Array[Float32], factor: Float32) -> Array[Float32]:
    return x.map(v -> v * factor)
```

Kernel function syntax:

```melt
kernel fn scale(x: Array[Float32], factor: Float32) -> Array[Float32]:
    return x.map(v -> v * factor)
```

Rules:

- `main` must exist exactly once
- `main` must be a host function
- `main` must take no parameters
- `main` must return `Void` implicitly

### Types

Currently usable types:

- `Bool`
- `Int64`
- `Float32`
- `Float64`
- `String`
- `Array[T]`
- user-defined `struct` types

Notes:

- `Float32` is the recommended type for GPU-oriented code in the current implementation
- `Float64` exists, but the benchmark/native GPU path is intentionally centered on `Float32`

#### Array Types

An array type is written as:

```melt
Array[Float32]
```

Examples:

```melt
Array[Int64]
Array[Float32]
Array[PriceRow]
```

### Variables And Assignment

Melt supports local bindings with or without an explicit type annotation.

Type-inferred binding:

```melt
rows = csv.load("data/prices_100k.csv", as=Array[PriceRow])
```

Explicitly typed binding:

```melt
factor: Float32 = 1.1
```

Assignment to an existing value is part of the grammar, but the current practical subset is centered on local bindings and direct expression flow.

### Expressions

Currently used expression forms include:

- names
- numeric literals
- string literals
- boolean literals
- function calls
- field access
- array map
- array filter
- numeric reductions
- arithmetic with numeric operands
- lambda expressions used by `map` and `filter`

Examples:

```melt
1
1.1
"prices.csv"
true
r.close
scale(close, 1.1)
x.map(v -> v * factor)
close.filter(v -> v > 105.0)
close.sum()
close.mean()
```

### Numeric Literals And Types

Current behavior:

- integer literals default to `Int64`
- floating-point literals default to `Float64`

Because of that, if you want a `Float32` factor in host code, it is clearer to write:

```melt
factor: Float32 = 1.1
```

For the GPU benchmark path, `Float32` is the intended “level playing field” numeric type.

### Field Access

Fields are accessed with dot syntax:

```melt
r.close
```

This is commonly used inside a `map` over an array of structs:

```melt
close = rows.map(r -> r.close)
```

### Function Calls

Calls use standard parentheses:

```melt
scale(close, 1.1)
print(out)
```

Named arguments are currently used in the CSV load API:

```melt
rows = csv.load("data/prices_100k.csv", as=Array[PriceRow])
```

### Lambdas

The implemented subset currently uses lambdas in `map`.

Single-argument lambda:

```melt
v -> v * factor
```

Examples:

```melt
rows.map(r -> r.close)
x.map(v -> v * factor)
```

### Statements

The practical implemented statement forms are:

- local binding
- return
- expression statement

Examples:

```melt
rows = csv.load("data/prices_100k.csv", as=Array[PriceRow])
out = scale(close, 1.1)
print(out)
return x.map(v -> v * factor)
```

`if` is part of the parser/sema surface, but the benchmark and example path is focused on straight-line programs.

### Host Domain

Host functions are regular `fn`.

Host code can currently do:

- call `print`
- call `csv.load`
- call `csv.col`
- call `csv.save`
- define and call host helper functions
- call `kernel fn`
- manipulate arrays in the supported map/filter/reduce subset on GPU-friendly numeric types

Example:

```melt
fn scale(x: Array[Float32], factor: Float32) -> Array[Float32]:
    return x.map(v -> v * factor)
```

### Kernel Domain

Kernel functions are written with `kernel fn`.

Example:

```melt
kernel fn scale(x: Array[Float32], factor: Float32) -> Array[Float32]:
    return x.map(v -> v * factor)
```

Current kernel-safe benchmark path:

- `Array[Float32]`
- `Float32`
- simple `map` shape

Current kernel restrictions in practice:

- no file IO
- no CSV APIs inside kernels
- no `print`
- no host orchestration inside kernels

### Built-In And Standard APIs

#### `print`

Prints a value from host code.

Example:

```melt
print(out)
```

#### `csv.load`

Loads CSV data into a typed array.

Current supported style:

```melt
rows = csv.load("data/prices_100k.csv", as=Array[PriceRow])
```

Expected usage:

- the CSV header should match the struct field names
- the `as=` argument must be a type
- the current examples use typed row structs

#### `csv.col`

Loads a single numeric CSV column directly into a GPU-friendly array type.

Current supported style:

```melt
close = csv.col("data/prices_10k.csv", "close", as=Float32)
```

Current v1 constraints:

- only `Float32` and `Int32`
- the named `as=` argument must be present
- the column name must match the CSV header exactly

#### `parquet.col`

Declares a numeric Parquet column load in the Melt language.

Current supported style:

```melt
close = parquet.col("data/prices_10k.parquet", "close", as=Float32)
```

Current v1 constraints:

- only `Float32` and `Int32`
- the named `as=` argument must be present
- the column name must match the Parquet column exactly

Current execution status:

- `meltc check` supports it
- `meltc mir` supports it
- `meltc build` supports it
- `meltc run` does not execute it yet
- the Swift runtime surface exists, but real Parquet file decoding is not implemented yet

#### `csv.save`

Saves a numeric array to CSV-like newline output.

Example:

```melt
csv.save("build/out.csv", out)
```

Current practical support is focused on numeric arrays produced by the implemented scale examples.

### Array Operations

#### `map`

The implemented array transform operation is `map`.

Example:

```melt
out = x.map(v -> v * factor)
```

Example over structs:

```melt
close = rows.map(r -> r.close)
```

This is the core operation behind both the CPU and GPU scale examples.

#### `filter`

`filter` keeps only the elements whose lambda predicate returns `true`.

Example:

```melt
movers = close.filter(v -> v > 105.0)
```

Current v1 constraints:

- only `Array[Float32]` and `Array[Int32]`
- the lambda must have exactly one parameter
- the lambda must be a direct comparison against a constant or named value

#### Aggregations

The implemented analytical reductions are:

- `sum()`
- `count()`
- `mean()`
- `min()`
- `max()`

Example:

```melt
count = movers.count()
total = movers.sum()
avg = movers.mean()
low = movers.min()
high = movers.max()
```

Current v1 constraints:

- only `Array[Float32]` and `Array[Int32]`
- `count()` returns `Int64`
- `mean()` returns `Float32` for `Array[Float32]` and `Float32` for `Array[Int32]`

### CPU And GPU Example Pair

The recommended benchmark pair now uses identical Melt code except for the `kernel` keyword.

CPU version:

```melt
struct PriceRow:
    ts: Int64
    close: Float32

fn scale(x: Array[Float32], factor: Float32) -> Array[Float32]:
    return x.map(v -> v * factor)

fn main():
    rows = csv.load("data/prices_100k.csv", as=Array[PriceRow])
    close = rows.map(r -> r.close)
    out = scale(close, 1.1)
    print(out)
    csv.save("build/out_cpu_scale_f32.csv", out)
```

GPU version:

```melt
struct PriceRow:
    ts: Int64
    close: Float32

kernel fn scale(x: Array[Float32], factor: Float32) -> Array[Float32]:
    return x.map(v -> v * factor)

fn main():
    rows = csv.load("data/prices_100k.csv", as=Array[PriceRow])
    close = rows.map(r -> r.close)
    out = scale(close, 1.1)
    print(out)
    csv.save("build/out_gpu_scale_f32.csv", out)
```

### What Is Implemented Reliably Today

The current reliable language slice is:

- `struct`
- host `fn`
- `kernel fn`
- typed parameters and return types
- local bindings
- `return`
- field projection
- `csv.load(..., as=Array[T])`
- `csv.col(path, column, as=Float32|Int32)`
- `parquet.col(path, column, as=Float32|Int32)` at the language/compiler level
- `csv.save(...)`
- `print(...)`
- array `map`
- array `filter`
- `sum`, `count`, `mean`, `min`, `max` on `Array[Float32]` and `Array[Int32]`
- the scale pattern used by the examples and benchmarks

Current note:

- the analytics API now exists in the compiler, MIR, interpreter, and native Swift path
- Parquet is now part of the language surface and lowers correctly, but runtime Parquet decoding is still pending
- keeping intermediate analytical buffers resident on the GPU across chained operators is the next execution-engine step

### What Is Not Yet A Full Stable Language Feature

These are either partial, in progress, or not the main supported path yet:

- a broad standard library
- general-purpose kernel programming beyond the current scale/map path
- complete native backend coverage for the broader PRD surface
- real Parquet decoding in the Swift runtime
- a fully standalone `meltc` binary that carries all runtime/build support with it

## Examples

Example source files:

- [examples/cpu_scale_f32.melt](/Users/gunesh/projects/melt-lang/examples/cpu_scale_f32.melt)
- [examples/gpu_scale_f32.melt](/Users/gunesh/projects/melt-lang/examples/gpu_scale_f32.melt)
- [examples/analytics_stats_f32.melt](/Users/gunesh/projects/melt-lang/examples/analytics_stats_f32.melt)
- [examples/parquet_stats_f32.melt](/Users/gunesh/projects/melt-lang/examples/parquet_stats_f32.melt)

CPU example:

```melt
struct PriceRow:
    ts: Int64
    close: Float32

fn scale(x: Array[Float32], factor: Float32) -> Array[Float32]:
    return x.map(v -> v * factor)

fn main():
    rows = csv.load("data/prices_100k.csv", as=Array[PriceRow])
    close = rows.map(r -> r.close)
    out = scale(close, 1.1)
    print(out)
    csv.save("build/out_cpu_scale_f32.csv", out)
```

GPU example:

```melt
struct PriceRow:
    ts: Int64
    close: Float32

kernel fn scale(x: Array[Float32], factor: Float32) -> Array[Float32]:
    return x.map(v -> v * factor)

fn main():
    rows = csv.load("data/prices_100k.csv", as=Array[PriceRow])
    close = rows.map(r -> r.close)
    out = scale(close, 1.1)
    print(out)
    csv.save("build/out_gpu_scale_f32.csv", out)
```

Analytics example:

```melt
fn main():
    close = csv.col("data/prices_10k.csv", "close", as=Float32)
    movers = close.filter(v -> v > 105.0)

    count = movers.count()
    total = movers.sum()
    avg = movers.mean()
    low = movers.min()
    high = movers.max()

    print(count)
    print(total)
    print(avg)
    print(low)
    print(high)
```

Verified commands:

```bash
./bin/meltc run examples/analytics_stats_f32.melt
./bin/meltc build examples/analytics_stats_f32.melt -o build/analytics_stats_f32
./build/analytics_stats_f32
```

Expected shape of results on `data/prices_10k.csv`:

- `count`: `4990`
- `sum`: about `536425`
- `mean`: about `107.5`
- `min`: `105.01`
- `max`: `109.99`

Parquet analytics example:

```melt
fn main():
    close = parquet.col("data/prices_10k.parquet", "close", as=Float32)
    movers = close.filter(v -> v > 105.0)

    count = movers.count()
    total = movers.sum()
    avg = movers.mean()
    low = movers.min()
    high = movers.max()

    print(count)
    print(total)
    print(avg)
    print(low)
    print(high)
```

Verified Parquet compiler results:

```bash
./bin/meltc check examples/parquet_stats_f32.melt
./bin/meltc mir examples/parquet_stats_f32.melt
./bin/meltc build examples/parquet_stats_f32.melt -o build/parquet_stats_f32
```

Current Parquet runtime status:

- `./bin/meltc run examples/parquet_stats_f32.melt` fails because the Go interpreter does not execute Parquet yet
- `./build/parquet_stats_f32` currently fails at runtime with `parquetDecodeError(...)` because the Swift runtime stub exists but real Parquet decoding is not implemented yet

Generate the sample dataset:

```bash
env GOCACHE="$PWD/build/.gocache" go run ./scripts/gen_prices_100k.go
```

Run the interpreter path:

```bash
./bin/meltc run tests/benchmarks/cpu_scale_100k_f32.melt
./bin/meltc run tests/benchmarks/gpu_scale_100k_f32.melt
```

Inspect MIR:

```bash
./bin/meltc mir examples/gpu_scale_f32.melt
```

Build native executables:

```bash
./bin/meltc build tests/benchmarks/cpu_scale_100k_f32.melt -o build/cpu_scale_100k_f32
./bin/meltc build tests/benchmarks/gpu_scale_100k_f32.melt -o build/gpu_scale_100k_f32
```

## Current Benchmark Comparison

I ran the current scale benchmarks on April 7, 2026. For each CPU/GPU pair, the Melt source is intentionally the same except for the `kernel` keyword on `scale`.

The key processing comparison is `compute_ms`. That is where the GPU does the actual array work, and it is the fairest CPU-vs-GPU comparison.

| Dataset | CPU `load_ms` | GPU `load_ms` | GPU `setup_ms` | CPU `compute_ms` | GPU `compute_ms` | GPU Compute Speedup | GPU `readback_ms` | CPU `save_ms` | GPU `save_ms` | CPU `total_ms` | GPU `total_ms` |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `10k` | `133.5460` | `126.9721` | `1.0409` | `0.8990` | `0.6239` | `1.44x faster` | `0.0360` | `10.9000` | `2.1271` | `145.3450` | `130.8000` |
| `1m` | `13017.1889` | `13312.6631` | `6.3490` | `79.5450` | `3.2620` | `24.39x faster` | `0.9201` | `176.1481` | `159.0450` | `13272.8820` | `13482.2392` |
| `10m` | `166852.1791` | `168195.2330` | `25.1470` | `822.7310` | `3.8459` | `213.93x faster` | `6.0511` | `1720.3150` | `1655.1260` | `169395.2250` | `169885.2769` |

Interpretation:

- the native comparison now uses `Float32` on both sides for a fairer benchmark path
- the processing area to focus on is `compute_ms`
- that is the closest CPU-vs-GPU comparison of the actual scale operation
- GPU-only stages such as `gpu_setup_ms` and `gpu_readback_ms` are overhead required to get the GPU result back to the host
- those overhead steps do not exist as separate stages in the CPU path, so only the GPU columns are populated for them
- at `10k`, the benchmark is small enough that overhead still matters, but GPU already wins end-to-end in this run
- at `1m` and `10m`, GPU compute is dramatically faster, but CSV load/save dominates the full program enough that total runtime remains close or slightly worse for GPU
- the current backend shows that GPU processing is faster, while full-program performance is still largely an IO problem

## Error Model

Melt currently reports errors in several broad categories.

### Compiler Errors

Lex errors:

- invalid character
- malformed number
- unterminated string
- tabs in indentation
- inconsistent indentation

Parse errors:

- malformed declaration
- malformed expression
- missing expected token
- malformed block structure

Resolution errors:

- unknown symbol
- duplicate symbol
- duplicate struct field
- unknown type
- unknown field on struct

Type errors:

- bad assignment type
- bad return type
- arithmetic on non-numeric values
- invalid field access
- indexing non-array values
- incorrect call argument count
- unsupported argument shape

Domain errors:

- invalid use of host-only operations in kernel code
- invalid `main` definition
- unsupported kernel-safe type usage

Lowering and backend errors:

- unsupported expression shape for current MIR lowering
- unsupported operation shape for code generation
- native Swift/Metal build failures

Runtime errors:

- missing CSV/data files
- CSV decode failures
- Metal unavailable
- missing metallib at runtime
- kernel dispatch/runtime failures

### Practical Meaning

For current users, the most common failure modes are:

- syntax/indentation mistakes in `.melt` files
- using language features outside the currently implemented subset
- missing input CSV files
- missing Apple toolchain pieces for native builds
- runtime Metal environment issues

## Try The Compiler

### Local Built Binary

The repo currently builds a local compiler binary at:

- [bin/meltc](/Users/gunesh/projects/melt-lang/bin/meltc)

Build it locally with:

```bash
mkdir -p bin
env GOCACHE="$PWD/build/.gocache" go build -o ./bin/meltc ./compiler/cmd/meltc
```

Then try:

```bash
./bin/meltc check examples/cpu_scale_f32.melt
./bin/meltc mir examples/gpu_scale_f32.melt
```

### GitHub Download

There is not yet a packaged GitHub release asset referenced by this README.

If you want a true “download and try” experience on GitHub, the next step is to publish platform-specific binaries as GitHub Release artifacts, for example:

- `meltc-darwin-arm64`

At that point the README can link directly to the release download URL.

### Is The Compiler Binary Alone Enough?

No, not for full usage.

The accurate answer is:

- `check`, `ast`, and `mir`: mostly yes, the user mainly needs the `meltc` binary and a `.melt` file
- `run`: no, the user also needs any runtime data files referenced by the program, such as CSV files
- `build`: no, the user also needs the Swift support sources in this repo plus the Apple Swift/Metal toolchain
- native GPU execution: no, the user also needs the generated `default.metallib` beside the executable

So I cannot honestly confirm that users only need the compiler binary to make Melt fully work today.

The compiler alone is enough only for:

- syntax checking
- AST inspection
- MIR inspection

It is not enough by itself for:

- interpreter runs that depend on external input data
- native executable builds
- native GPU execution

## Can Users Use Only The `meltc` Binary?

Not by itself in the current form.

Users can use the `meltc` binary alone only if all they need is the command executable. Real use still depends on other files:

- the input `.melt` source files
- CSV input data for the current examples
- the repo support sources for native build generation
- Apple toolchain components for Swift/Metal builds

More specifically:

- `meltc check`, `ast`, and `mir` only need the `meltc` binary plus the `.melt` file
- `meltc run` needs the binary, the `.melt` file, and any runtime input files referenced by that program such as CSV files
- `meltc build` also depends on the Swift support files in `support/Sources/MeltSupport/` and the Apple build tools

So today, the answer is:

- for analysis commands, almost yes
- for real execution, no
- for native builds, definitely no

## Project Layout

```text
compiler/   Go compiler source
support/    Swift support library sources
examples/   Example Melt programs
tests/      Program and benchmark inputs
data/       Sample datasets
bin/        Built compiler binaries
```

## License

MIT

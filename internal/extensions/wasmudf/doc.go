// Package wasmudf provides WASM-based user-defined function execution for
// custom feature transformations in any language that compiles to WebAssembly,
// with sandbox isolation and resource limits.
//
// Key components:
//   - Runtime: Manages WASM module registration and execution
//   - Module: Represents a registered WASM transformation function
//   - ExecutionResult: Captures output, timing, and resource usage
package wasmudf

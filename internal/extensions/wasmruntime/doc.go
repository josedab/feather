// Package wasmruntime provides a serverless edge runtime using WebAssembly
// for deploying feature computation at the edge with minimal footprint and
// offline-first capabilities. It supports module deployment, execution,
// and synchronization with the central feature store.
//
// Key components:
//   - EdgeManager: Manages edge device fleet and module deployment
//   - Module: A WASM module for edge feature computation
//   - Device: Represents an edge device with sync state
package wasmruntime

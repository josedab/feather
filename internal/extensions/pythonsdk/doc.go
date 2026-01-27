// Package pythonsdk provides a bridge between Python-defined feature
// transformations and Feather's Go serving layer. It enables data scientists
// to write feature logic in Python while Feather handles serving in Go with
// sub-millisecond latency.
//
// Key components:
//   - TransformRegistry: Registers and manages Python transform definitions
//   - TransformExecutor: Executes transforms via gRPC bridge to Python workers
//   - TransformDef: Declarative feature transformation definition
package pythonsdk

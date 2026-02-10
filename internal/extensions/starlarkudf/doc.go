// Package starlarkudf provides a lightweight embedded expression evaluator for
// user-defined feature transformations at serving time. It supports a Python-like
// syntax covering arithmetic, string operations, conditionals, and common math
// functions — targeting <5ms overhead for typical transformations.
//
// For full CPython support (numpy/pandas), use the gRPC sidecar protocol defined
// in the SidecarClient type.
package starlarkudf

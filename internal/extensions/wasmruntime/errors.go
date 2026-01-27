package wasmruntime

import "errors"

var (
	ErrModuleNotFound  = errors.New("wasm module not found")
	ErrModuleExists    = errors.New("wasm module already exists")
	ErrDeviceNotFound  = errors.New("edge device not found")
	ErrDeviceExists    = errors.New("edge device already exists")
	ErrInvalidModule   = errors.New("invalid wasm module")
	ErrDeviceOffline   = errors.New("device is offline")
)

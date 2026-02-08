package storage

import (
	"sync"
	"sync/atomic"
)

// Arena provides efficient memory allocation for feature values.
// It pre-allocates chunks to reduce GC pressure.
type Arena struct {
	chunks    [][]byte
	offset    int
	chunkSize int
	allocated int64
	mu        sync.Mutex
}

// NewArena creates a new arena with the given chunk size.
func NewArena(chunkSize int) *Arena {
	return &Arena{
		chunks:    make([][]byte, 0, 16),
		chunkSize: chunkSize,
	}
}

// Alloc allocates n bytes from the arena.
func (a *Arena) Alloc(n int) []byte {
	a.mu.Lock()
	defer a.mu.Unlock()

	if n > a.chunkSize {
		// For large allocations, create a dedicated chunk
		chunk := make([]byte, n)
		a.chunks = append(a.chunks, chunk)
		atomic.AddInt64(&a.allocated, int64(n))
		return chunk
	}

	// Check if we need a new chunk
	if len(a.chunks) == 0 || a.offset+n > len(a.chunks[len(a.chunks)-1]) {
		chunk := make([]byte, a.chunkSize)
		a.chunks = append(a.chunks, chunk)
		a.offset = 0
		atomic.AddInt64(&a.allocated, int64(a.chunkSize))
	}

	chunk := a.chunks[len(a.chunks)-1]
	data := chunk[a.offset : a.offset+n]
	a.offset += n
	return data
}

// Allocated returns the total bytes allocated.
func (a *Arena) Allocated() int64 {
	return atomic.LoadInt64(&a.allocated)
}

// Reset releases all allocated memory.
func (a *Arena) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.chunks = a.chunks[:0]
	a.offset = 0
	atomic.StoreInt64(&a.allocated, 0)
}

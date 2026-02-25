package storage

import (
	"sync"
	"testing"
)

func TestArena_AllocWithinChunk(t *testing.T) {
	a := NewArena(1024)
	data := a.Alloc(100)
	if len(data) != 100 {
		t.Errorf("expected 100 bytes, got %d", len(data))
	}
}

func TestArena_AllocMultipleSmallPacking(t *testing.T) {
	a := NewArena(1024)
	d1 := a.Alloc(100)
	d2 := a.Alloc(100)
	d3 := a.Alloc(100)

	if len(d1) != 100 || len(d2) != 100 || len(d3) != 100 {
		t.Error("expected all allocations to return 100 bytes")
	}

	// All should fit in a single chunk
	if a.Allocated() != 1024 {
		t.Errorf("expected 1024 allocated (one chunk), got %d", a.Allocated())
	}
}

func TestArena_AllocCrossingChunkBoundary(t *testing.T) {
	a := NewArena(256)
	a.Alloc(200)  // fits in chunk 1
	a.Alloc(100)  // exceeds chunk 1 → triggers chunk 2

	if a.Allocated() != 512 { // 256 + 256
		t.Errorf("expected 512 allocated (two chunks), got %d", a.Allocated())
	}
}

func TestArena_AllocLargerThanChunkSize(t *testing.T) {
	a := NewArena(256)
	data := a.Alloc(512)

	if len(data) != 512 {
		t.Errorf("expected 512 bytes, got %d", len(data))
	}
	if a.Allocated() != 512 {
		t.Errorf("expected 512 allocated, got %d", a.Allocated())
	}
}

func TestArena_AllocExactChunkSize(t *testing.T) {
	a := NewArena(256)
	data := a.Alloc(256)

	if len(data) != 256 {
		t.Errorf("expected 256 bytes, got %d", len(data))
	}
}

func TestArena_AllocZeroBytes(t *testing.T) {
	a := NewArena(256)
	data := a.Alloc(0)

	if len(data) != 0 {
		t.Errorf("expected 0 bytes, got %d", len(data))
	}
}

func TestArena_AllocatedTracking(t *testing.T) {
	a := NewArena(1024)
	if a.Allocated() != 0 {
		t.Errorf("expected 0 initially, got %d", a.Allocated())
	}

	a.Alloc(100)
	if a.Allocated() != 1024 {
		t.Errorf("expected 1024 after first alloc, got %d", a.Allocated())
	}

	a.Alloc(2000) // larger than chunk
	if a.Allocated() != 3024 { // 1024 + 2000
		t.Errorf("expected 3024, got %d", a.Allocated())
	}
}

func TestArena_Reset(t *testing.T) {
	a := NewArena(1024)
	a.Alloc(100)
	a.Alloc(200)

	a.Reset()

	if a.Allocated() != 0 {
		t.Errorf("expected 0 after reset, got %d", a.Allocated())
	}

	// Should be able to allocate again after reset
	data := a.Alloc(50)
	if len(data) != 50 {
		t.Errorf("expected 50 bytes after reset, got %d", len(data))
	}
}

func TestArena_ConcurrentAlloc(t *testing.T) {
	a := NewArena(256)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data := a.Alloc(10)
			if len(data) != 10 {
				t.Errorf("expected 10 bytes, got %d", len(data))
			}
		}()
	}
	wg.Wait()

	if a.Allocated() <= 0 {
		t.Error("expected positive allocation after concurrent allocs")
	}
}

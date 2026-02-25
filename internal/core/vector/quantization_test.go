package vector

import (
	"math"
	"sync"
	"testing"
)

// --- ScalarQuantizer ---

func TestScalarQuantizer_TrainEncodeDecode(t *testing.T) {
	sq := NewScalarQuantizer(3)
	vectors := [][]float32{
		{0.0, 1.0, 2.0},
		{3.0, 4.0, 5.0},
		{1.5, 2.5, 3.5},
	}
	sq.Train(vectors)

	original := []float32{1.5, 2.5, 3.5}
	encoded := sq.Encode("v1", original)
	decoded := sq.Decode(encoded)

	for i, v := range decoded {
		if math.Abs(float64(v-original[i])) > 0.1 {
			t.Errorf("dimension %d: decoded %.4f too far from original %.4f", i, v, original[i])
		}
	}
}

func TestScalarQuantizer_EmptyTraining(t *testing.T) {
	sq := NewScalarQuantizer(3)
	sq.Train(nil)

	if sq.trained {
		t.Error("expected trained=false after empty training")
	}
}

func TestScalarQuantizer_SingleDimension(t *testing.T) {
	sq := NewScalarQuantizer(1)
	sq.Train([][]float32{{0.0}, {10.0}})

	encoded := sq.Encode("v1", []float32{5.0})
	decoded := sq.Decode(encoded)

	if math.Abs(float64(decoded[0]-5.0)) > 0.2 {
		t.Errorf("expected ~5.0, got %.4f", decoded[0])
	}
}

func TestScalarQuantizer_AllSameValues(t *testing.T) {
	sq := NewScalarQuantizer(2)
	sq.Train([][]float32{{3.0, 3.0}, {3.0, 3.0}})

	encoded := sq.Encode("v1", []float32{3.0, 3.0})
	decoded := sq.Decode(encoded)

	// When range is 0, encoded values should be 0, decoded should be mins
	for i, v := range decoded {
		if v != 3.0 {
			t.Errorf("dim %d: expected 3.0, got %.4f", i, v)
		}
	}
}

func TestScalarQuantizer_MemoryUsage(t *testing.T) {
	sq := NewScalarQuantizer(4)
	sq.Train([][]float32{{0, 0, 0, 0}, {1, 1, 1, 1}})

	sq.Encode("a", []float32{0.5, 0.5, 0.5, 0.5})
	sq.Encode("b", []float32{0.1, 0.1, 0.1, 0.1})

	usage := sq.MemoryUsage()
	if usage != 8 { // 2 vectors * 4 dims * 1 byte
		t.Errorf("expected 8 bytes, got %d", usage)
	}
}

func TestScalarQuantizer_ConcurrentAccess(t *testing.T) {
	sq := NewScalarQuantizer(4)
	sq.Train([][]float32{{0, 0, 0, 0}, {10, 10, 10, 10}})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			v := float32(id)
			vec := []float32{v, v, v, v}
			encoded := sq.Encode(string(rune('A'+id)), vec)
			sq.Decode(encoded)
		}(i)
	}
	wg.Wait()
}

// --- ProductQuantizer ---

func TestProductQuantizer_TrainEncodeDecode(t *testing.T) {
	pq := NewProductQuantizer(8, 2, 4)
	vectors := make([][]float32, 10)
	for i := range vectors {
		vectors[i] = make([]float32, 8)
		for j := range vectors[i] {
			vectors[i][j] = float32(i*8 + j)
		}
	}
	pq.Train(vectors)

	original := vectors[0]
	codes := pq.Encode("v0", original)
	decoded := pq.Decode(codes)

	if len(decoded) != 8 {
		t.Fatalf("expected 8 dims, got %d", len(decoded))
	}
}

func TestProductQuantizer_EmptyTraining(t *testing.T) {
	pq := NewProductQuantizer(4, 2, 4)
	pq.Train(nil)

	if pq.trained {
		t.Error("expected trained=false after empty training")
	}
}

func TestProductQuantizer_DimNotDivisibleByNumSubs(t *testing.T) {
	// dim=5, numSubs=3 → subDim=1, numSubs adjusted to 5
	pq := NewProductQuantizer(5, 3, 4)
	vectors := [][]float32{
		{1, 2, 3, 4, 5},
		{6, 7, 8, 9, 10},
	}
	pq.Train(vectors)

	codes := pq.Encode("v1", vectors[0])
	decoded := pq.Decode(codes)

	if len(decoded) != 5 {
		t.Errorf("expected 5 dims, got %d", len(decoded))
	}
}

func TestProductQuantizer_DefaultParams(t *testing.T) {
	pq := NewProductQuantizer(8, 0, 0)
	if pq.numSubs != 8 {
		t.Errorf("expected default numSubs=8, got %d", pq.numSubs)
	}
	if pq.numCentroids != 256 {
		t.Errorf("expected default numCentroids=256, got %d", pq.numCentroids)
	}
}

// --- IVFIndex ---

func TestIVFIndex_TrainAssignSearch(t *testing.T) {
	ivf := NewIVFIndex(3, 4)
	vectors := [][]float32{
		{0, 0, 0},
		{1, 1, 1},
		{10, 10, 10},
		{11, 11, 11},
	}
	ivf.Train(vectors)

	ivf.Assign("a", []float32{0.1, 0.1, 0.1})
	ivf.Assign("b", []float32{10.1, 10.1, 10.1})

	partitions := ivf.SearchPartitions([]float32{0, 0, 0}, 2)
	if len(partitions) != 2 {
		t.Fatalf("expected 2 partitions, got %d", len(partitions))
	}
}

func TestIVFIndex_EmptyTraining(t *testing.T) {
	ivf := NewIVFIndex(3, 4)
	ivf.Train(nil)

	if ivf.trained {
		t.Error("expected trained=false after empty training")
	}
}

func TestIVFIndex_ReassignVector(t *testing.T) {
	ivf := NewIVFIndex(2, 2)
	ivf.Train([][]float32{{0, 0}, {10, 10}})

	p1 := ivf.Assign("v1", []float32{0, 0})
	ids := ivf.GetPartitionIDs(p1)
	if len(ids) != 1 || ids[0] != "v1" {
		t.Errorf("expected [v1] in partition %d, got %v", p1, ids)
	}

	// Reassign to different partition
	p2 := ivf.Assign("v1", []float32{10, 10})
	_ = p2

	// Old partition should no longer have v1
	idsOld := ivf.GetPartitionIDs(p1)
	for _, id := range idsOld {
		if id == "v1" {
			t.Error("v1 should have been removed from old partition")
		}
	}
}

func TestIVFIndex_GetPartitionIDs_Empty(t *testing.T) {
	ivf := NewIVFIndex(3, 4)
	ids := ivf.GetPartitionIDs(99)
	if len(ids) != 0 {
		t.Errorf("expected empty, got %v", ids)
	}
}

func TestIVFIndex_SearchPartitions_Clamp(t *testing.T) {
	ivf := NewIVFIndex(2, 2)
	ivf.Train([][]float32{{0, 0}, {10, 10}})

	// nProbe > nList should be clamped
	partitions := ivf.SearchPartitions([]float32{5, 5}, 100)
	if len(partitions) != 2 {
		t.Errorf("expected 2 partitions (clamped), got %d", len(partitions))
	}

	// nProbe <= 0 should default to 1
	partitions = ivf.SearchPartitions([]float32{5, 5}, 0)
	if len(partitions) != 1 {
		t.Errorf("expected 1 partition (default), got %d", len(partitions))
	}
}

func TestIVFIndex_DefaultNList(t *testing.T) {
	ivf := NewIVFIndex(3, 0)
	if ivf.nList != 16 {
		t.Errorf("expected default nList=16, got %d", ivf.nList)
	}
}

func TestIVFIndex_ConcurrentAccess(t *testing.T) {
	ivf := NewIVFIndex(3, 4)
	ivf.Train([][]float32{{0, 0, 0}, {5, 5, 5}, {10, 10, 10}, {15, 15, 15}})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			v := float32(id)
			ivf.Assign(string(rune('A'+id)), []float32{v, v, v})
			ivf.SearchPartitions([]float32{v, v, v}, 2)
		}(i)
	}
	wg.Wait()
}

// --- GetAcceleratorInfo ---

func TestGetAcceleratorInfo_CPU(t *testing.T) {
	info := GetAcceleratorInfo(DefaultAcceleratorConfig())
	if info.Backend != AccelCPU {
		t.Errorf("expected CPU backend, got %s", info.Backend)
	}
	if !info.Available {
		t.Error("expected Available=true")
	}
}

func TestGetAcceleratorInfo_CUDA(t *testing.T) {
	info := GetAcceleratorInfo(AcceleratorConfig{Type: AccelCUDA})
	if info.Backend != AccelCPU {
		t.Errorf("expected CPU fallback, got %s", info.Backend)
	}
	if info.Message == "" {
		t.Error("expected fallback message")
	}
}

func TestGetAcceleratorInfo_Metal(t *testing.T) {
	info := GetAcceleratorInfo(AcceleratorConfig{Type: AccelMetal})
	if info.Backend != AccelCPU {
		t.Errorf("expected CPU fallback, got %s", info.Backend)
	}
}

func TestGetAcceleratorInfo_Auto(t *testing.T) {
	info := GetAcceleratorInfo(AcceleratorConfig{Type: AccelAuto})
	if info.Message != "Auto-detected CPU backend" {
		t.Errorf("unexpected message: %s", info.Message)
	}
}

func TestGetAcceleratorInfo_ScalarQuantization(t *testing.T) {
	info := GetAcceleratorInfo(AcceleratorConfig{Quantization: QuantScalar})
	if info.MemorySaved != "~75% (float32 → uint8)" {
		t.Errorf("unexpected memory saved: %s", info.MemorySaved)
	}
}

func TestGetAcceleratorInfo_ProductQuantization(t *testing.T) {
	info := GetAcceleratorInfo(AcceleratorConfig{Quantization: QuantProduct})
	if info.MemorySaved != "~90%+ (product quantization)" {
		t.Errorf("unexpected memory saved: %s", info.MemorySaved)
	}
}

func TestGetAcceleratorInfo_IVFPartitions(t *testing.T) {
	info := GetAcceleratorInfo(AcceleratorConfig{IVFPartitions: 16})
	if info.IVFPartitions != 16 {
		t.Errorf("expected 16 IVF partitions, got %d", info.IVFPartitions)
	}
}

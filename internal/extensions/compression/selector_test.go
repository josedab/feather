package compression

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func makeIntData(values []int64) []byte {
	buf := new(bytes.Buffer)
	for _, v := range values {
		binary.Write(buf, binary.LittleEndian, v)
	}
	return buf.Bytes()
}

func makeFloatData(values []float64) []byte {
	buf := new(bytes.Buffer)
	for _, v := range values {
		binary.Write(buf, binary.LittleEndian, v)
	}
	return buf.Bytes()
}

func TestNewSelector(t *testing.T) {
	s := NewSelector(DefaultConfig())
	if s == nil {
		t.Fatal("NewSelector returned nil")
	}
	stats := s.Stats()
	if stats.TotalCompressed != 0 {
		t.Errorf("TotalCompressed = %d, want 0", stats.TotalCompressed)
	}
}

func TestAnalyzeData(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		dataType string
		wantErr  bool
	}{
		{
			name:     "int data",
			data:     makeIntData([]int64{1, 2, 3, 4, 5}),
			dataType: "int",
		},
		{
			name:     "float data",
			data:     makeFloatData([]float64{1.1, 2.2, 3.3}),
			dataType: "float",
		},
		{
			name:     "string data",
			data:     []byte("hello world hello"),
			dataType: "string",
		},
		{
			name:     "timestamp data",
			data:     makeIntData([]int64{1000, 2000, 3000}),
			dataType: "timestamp",
		},
		{
			name:     "empty data",
			data:     []byte{},
			dataType: "int",
			wantErr:  true,
		},
		{
			name:     "misaligned int data",
			data:     []byte{1, 2, 3},
			dataType: "int",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats, err := AnalyzeData(tt.data, tt.dataType)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if stats.Size != int64(len(tt.data)) {
				t.Errorf("Size = %d, want %d", stats.Size, len(tt.data))
			}
		})
	}
}

func TestSelectStrategy(t *testing.T) {
	tests := []struct {
		name     string
		stats    *DataStats
		expected Strategy
	}{
		{
			name:     "nil stats",
			stats:    nil,
			expected: StrategyNone,
		},
		{
			name:     "low cardinality",
			stats:    &DataStats{Cardinality: 10, Size: 1000},
			expected: StrategyDictionary,
		},
		{
			name:     "temporal pattern",
			stats:    &DataStats{Cardinality: 1000, TemporalPattern: true, Size: 1000},
			expected: StrategyGorilla,
		},
		{
			name:     "high repeat rate",
			stats:    &DataStats{Cardinality: 500, RepeatRate: 0.8, Size: 1000},
			expected: StrategyRLE,
		},
		{
			name:     "small data",
			stats:    &DataStats{Cardinality: 500, Size: 32},
			expected: StrategyNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SelectStrategy(tt.stats)
			if got != tt.expected {
				t.Errorf("SelectStrategy = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestCompressDecompress(t *testing.T) {
	strategies := []struct {
		name     Strategy
		data     []byte
	}{
		{StrategyDictionary, []byte("aabbbccccdddddeeeee")},
		{StrategyDelta, []byte{10, 11, 12, 13, 14, 15}},
		{StrategyRLE, bytes.Repeat([]byte{0xAA}, 50)},
		{StrategyXOR, []byte{100, 101, 102, 103, 104}},
		{StrategyLZ4, bytes.Repeat([]byte("abcdef"), 20)},
		{StrategyNone, []byte("raw data")},
		{StrategyGorilla, []byte{10, 11, 12, 13, 14}},
	}

	for _, tt := range strategies {
		t.Run(string(tt.name), func(t *testing.T) {
			s := NewSelector(DefaultConfig())

			block, err := s.Compress(tt.data, tt.name)
			if err != nil {
				t.Fatalf("Compress: %v", err)
			}
			if block.OriginalSize != int64(len(tt.data)) {
				t.Errorf("OriginalSize = %d, want %d", block.OriginalSize, len(tt.data))
			}
			if block.Strategy != tt.name {
				t.Errorf("Strategy = %q, want %q", block.Strategy, tt.name)
			}

			decompressed, err := s.Decompress(block)
			if err != nil {
				t.Fatalf("Decompress: %v", err)
			}
			if !bytes.Equal(decompressed, tt.data) {
				t.Errorf("round-trip failed: got %v, want %v", decompressed, tt.data)
			}
		})
	}
}

func TestCompress_EmptyData(t *testing.T) {
	s := NewSelector(DefaultConfig())
	_, err := s.Compress([]byte{}, StrategyNone)
	if err == nil {
		t.Error("expected error for empty data")
	}
}

func TestCompress_UnknownStrategy(t *testing.T) {
	s := NewSelector(DefaultConfig())
	_, err := s.Compress([]byte("data"), Strategy("unknown"))
	if err == nil {
		t.Error("expected error for unknown strategy")
	}
}

func TestDecompress_NilBlock(t *testing.T) {
	s := NewSelector(DefaultConfig())
	_, err := s.Decompress(nil)
	if err == nil {
		t.Error("expected error for nil block")
	}
}

func TestStats(t *testing.T) {
	s := NewSelector(DefaultConfig())
	data := []byte("hello world hello world hello world")

	s.Compress(data, StrategyDictionary)
	s.Compress(data, StrategyRLE)

	stats := s.Stats()
	if stats.TotalCompressed != 2 {
		t.Errorf("TotalCompressed = %d, want 2", stats.TotalCompressed)
	}
	if stats.AvgRatio <= 0 {
		t.Error("AvgRatio should be positive")
	}
	if len(stats.StrategyDistribution) == 0 {
		t.Error("StrategyDistribution should not be empty")
	}
}

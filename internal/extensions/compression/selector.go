// Package compression provides an intelligent tiered compression engine
// that analyzes per-feature data characteristics and selects optimal encoding.
package compression

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

// Strategy represents a compression encoding strategy.
type Strategy string

const (
	StrategyDictionary Strategy = "dictionary"
	StrategyDelta      Strategy = "delta"
	StrategyRLE        Strategy = "rle"
	StrategyGorilla    Strategy = "gorilla"
	StrategyXOR        Strategy = "xor"
	StrategyLZ4        Strategy = "lz4"
	StrategyNone       Strategy = "none"
)

// Config holds tuning parameters for the compression selector.
type Config struct {
	TargetCompressionRatio float64
	MaxCompressionTimeMs   int64
	EnableAdaptive         bool
	BenchmarkSampleSize    int
	ReEncodingThreshold    float64
}

// DefaultConfig returns a Config with sensible production defaults.
func DefaultConfig() Config {
	return Config{
		TargetCompressionRatio: 4.0,
		MaxCompressionTimeMs:   100,
		EnableAdaptive:         true,
		BenchmarkSampleSize:    1000,
		ReEncodingThreshold:    0.2,
	}
}

// DataStats summarises the statistical properties of a data sample.
type DataStats struct {
	Cardinality     int
	ValueRange      float64
	TemporalPattern bool
	RepeatRate      float64
	MeanValue       float64
	StdDev          float64
	Size            int64
}

// CompressedBlock is the output of a compression operation.
type CompressedBlock struct {
	Data           []byte
	Strategy       Strategy
	OriginalSize   int64
	CompressedSize int64
	Ratio          float64
}

// compressionRecord tracks a single compression result for adaptive re-encoding.
type compressionRecord struct {
	strategy Strategy
	ratio    float64
	ts       time.Time
}

// featureHistory stores the compression history for a single feature.
type featureHistory struct {
	records []compressionRecord
}

// SelectorStats exposes aggregate compression metrics.
type SelectorStats struct {
	TotalCompressed      int64
	TotalDecompressed    int64
	AvgRatio             float64
	StrategyDistribution map[Strategy]int
}

// Selector analyses data and selects the optimal compression strategy.
type Selector struct {
	cfg Config

	mu       sync.RWMutex
	history  map[string]*featureHistory
	stratDist map[Strategy]int
	totalComp   int64
	totalDecomp int64
	ratioSum    float64
	ratioCount  int64
}

// NewSelector creates a Selector with the given configuration.
func NewSelector(cfg Config) *Selector {
	return &Selector{
		cfg:       cfg,
		history:   make(map[string]*featureHistory),
		stratDist: make(map[Strategy]int),
	}
}

// AnalyzeData computes statistical properties of data for strategy selection.
// dataType should be one of "int", "float", "string", "timestamp", "bytes".
func AnalyzeData(data []byte, dataType string) (*DataStats, error) {
	if len(data) == 0 {
		return nil, errors.New("compression: empty data")
	}

	stats := &DataStats{Size: int64(len(data))}

	switch dataType {
	case "int":
		if err := analyzeInts(data, stats); err != nil {
			return nil, fmt.Errorf("compression: analyzing ints: %w", err)
		}
	case "float":
		if err := analyzeFloats(data, stats); err != nil {
			return nil, fmt.Errorf("compression: analyzing floats: %w", err)
		}
	case "timestamp":
		if err := analyzeTimestamps(data, stats); err != nil {
			return nil, fmt.Errorf("compression: analyzing timestamps: %w", err)
		}
	case "string", "bytes":
		analyzeBytes(data, stats)
	default:
		analyzeBytes(data, stats)
	}

	return stats, nil
}

// analyzeInts extracts statistics from int64-encoded data.
func analyzeInts(data []byte, stats *DataStats) error {
	if len(data)%8 != 0 {
		return errors.New("data length not aligned to int64")
	}
	n := len(data) / 8
	if n == 0 {
		return errors.New("no values")
	}

	seen := make(map[int64]int)
	vals := make([]int64, n)
	r := bytes.NewReader(data)
	for i := 0; i < n; i++ {
		if err := binary.Read(r, binary.LittleEndian, &vals[i]); err != nil {
			return err
		}
		seen[vals[i]]++
	}

	stats.Cardinality = len(seen)

	var sum float64
	minV, maxV := vals[0], vals[0]
	for _, v := range vals {
		f := float64(v)
		sum += f
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	stats.MeanValue = sum / float64(n)
	stats.ValueRange = float64(maxV - minV)

	var sqDiff float64
	for _, v := range vals {
		d := float64(v) - stats.MeanValue
		sqDiff += d * d
	}
	stats.StdDev = math.Sqrt(sqDiff / float64(n))

	// Detect monotonic increase → temporal pattern.
	mono := true
	for i := 1; i < n; i++ {
		if vals[i] < vals[i-1] {
			mono = false
			break
		}
	}
	stats.TemporalPattern = mono && n > 1

	// Repeat rate: fraction of values that are duplicates.
	var repeats int
	for _, c := range seen {
		if c > 1 {
			repeats += c - 1
		}
	}
	stats.RepeatRate = float64(repeats) / float64(n)

	return nil
}

// analyzeFloats extracts statistics from float64-encoded data.
func analyzeFloats(data []byte, stats *DataStats) error {
	if len(data)%8 != 0 {
		return errors.New("data length not aligned to float64")
	}
	n := len(data) / 8
	if n == 0 {
		return errors.New("no values")
	}

	seen := make(map[uint64]int)
	vals := make([]float64, n)
	r := bytes.NewReader(data)
	for i := 0; i < n; i++ {
		if err := binary.Read(r, binary.LittleEndian, &vals[i]); err != nil {
			return err
		}
		seen[math.Float64bits(vals[i])]++
	}

	stats.Cardinality = len(seen)

	var sum float64
	minV, maxV := vals[0], vals[0]
	for _, v := range vals {
		sum += v
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	stats.MeanValue = sum / float64(n)
	stats.ValueRange = maxV - minV

	var sqDiff float64
	for _, v := range vals {
		d := v - stats.MeanValue
		sqDiff += d * d
	}
	stats.StdDev = math.Sqrt(sqDiff / float64(n))

	mono := true
	for i := 1; i < n; i++ {
		if vals[i] < vals[i-1] {
			mono = false
			break
		}
	}
	stats.TemporalPattern = mono && n > 1

	var repeats int
	for _, c := range seen {
		if c > 1 {
			repeats += c - 1
		}
	}
	stats.RepeatRate = float64(repeats) / float64(n)

	return nil
}

// analyzeTimestamps analyses int64 unix-nanos data and marks temporal patterns.
func analyzeTimestamps(data []byte, stats *DataStats) error {
	if err := analyzeInts(data, stats); err != nil {
		return err
	}
	stats.TemporalPattern = true
	return nil
}

// analyzeBytes computes byte-level statistics for opaque data.
func analyzeBytes(data []byte, stats *DataStats) {
	seen := make(map[byte]int)
	for _, b := range data {
		seen[b]++
	}
	stats.Cardinality = len(seen)
	stats.RepeatRate = 1.0 - float64(len(seen))/256.0
	if stats.RepeatRate < 0 {
		stats.RepeatRate = 0
	}
}

// SelectStrategy picks the best strategy based on data statistics.
func SelectStrategy(stats *DataStats) Strategy {
	if stats == nil {
		return StrategyNone
	}

	// Low cardinality → dictionary encoding.
	if stats.Cardinality > 0 && stats.Cardinality <= 256 && stats.Size > 64 {
		return StrategyDictionary
	}

	// Temporal / monotonically increasing → gorilla.
	if stats.TemporalPattern {
		return StrategyGorilla
	}

	// High repeat rate → run-length encoding.
	if stats.RepeatRate >= 0.5 {
		return StrategyRLE
	}

	// Low standard deviation relative to mean → delta encoding.
	if stats.MeanValue != 0 && stats.StdDev/math.Abs(stats.MeanValue) < 0.01 {
		return StrategyDelta
	}

	// Floating-point-like value range → XOR encoding.
	if stats.StdDev > 0 && stats.ValueRange > 0 && stats.ValueRange/stats.StdDev > 10 {
		return StrategyXOR
	}

	// Fall back to general-purpose.
	if stats.Size > 64 {
		return StrategyLZ4
	}

	return StrategyNone
}

// Compress compresses data using the given strategy.
func (s *Selector) Compress(data []byte, strategy Strategy) (*CompressedBlock, error) {
	if len(data) == 0 {
		return nil, errors.New("compression: empty data")
	}

	start := time.Now()

	var compressed []byte
	var err error

	switch strategy {
	case StrategyDictionary:
		compressed, err = compressDictionary(data)
	case StrategyDelta:
		compressed, err = compressDelta(data)
	case StrategyRLE:
		compressed, err = compressRLE(data)
	case StrategyGorilla:
		compressed, err = compressDelta(data) // simplified gorilla via delta
	case StrategyXOR:
		compressed, err = compressXOR(data)
	case StrategyLZ4:
		compressed, err = compressLZ4(data)
	case StrategyNone:
		compressed = make([]byte, len(data))
		copy(compressed, data)
	default:
		return nil, fmt.Errorf("compression: unknown strategy %q", strategy)
	}
	if err != nil {
		return nil, fmt.Errorf("compression: %s: %w", strategy, err)
	}

	elapsed := time.Since(start).Milliseconds()
	if s.cfg.MaxCompressionTimeMs > 0 && elapsed > s.cfg.MaxCompressionTimeMs {
		// Fallback to no compression when the budget is exceeded.
		compressed = make([]byte, len(data))
		copy(compressed, data)
		strategy = StrategyNone
	}

	origSize := int64(len(data))
	compSize := int64(len(compressed))
	ratio := float64(origSize) / float64(compSize)
	if compSize == 0 {
		ratio = 1.0
	}

	block := &CompressedBlock{
		Data:           compressed,
		Strategy:       strategy,
		OriginalSize:   origSize,
		CompressedSize: compSize,
		Ratio:          ratio,
	}

	s.mu.Lock()
	s.totalComp++
	s.ratioSum += ratio
	s.ratioCount++
	s.stratDist[strategy]++
	s.mu.Unlock()

	return block, nil
}

// Decompress restores the original data from a CompressedBlock.
func (s *Selector) Decompress(block *CompressedBlock) ([]byte, error) {
	if block == nil {
		return nil, errors.New("compression: nil block")
	}

	var out []byte
	var err error

	switch block.Strategy {
	case StrategyDictionary:
		out, err = decompressDictionary(block.Data)
	case StrategyDelta:
		out, err = decompressDelta(block.Data)
	case StrategyRLE:
		out, err = decompressRLE(block.Data)
	case StrategyGorilla:
		out, err = decompressDelta(block.Data)
	case StrategyXOR:
		out, err = decompressXOR(block.Data)
	case StrategyLZ4:
		out, err = decompressLZ4(block.Data, block.OriginalSize)
	case StrategyNone:
		out = make([]byte, len(block.Data))
		copy(out, block.Data)
	default:
		return nil, fmt.Errorf("compression: unknown strategy %q", block.Strategy)
	}
	if err != nil {
		return nil, fmt.Errorf("compression: decompress %s: %w", block.Strategy, err)
	}

	s.mu.Lock()
	s.totalDecomp++
	s.mu.Unlock()

	return out, nil
}

// RecordCompressionResult tracks a compression outcome for adaptive re-encoding.
func (s *Selector) RecordCompressionResult(feature string, block *CompressedBlock) {
	if block == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	h, ok := s.history[feature]
	if !ok {
		h = &featureHistory{}
		s.history[feature] = h
	}
	h.records = append(h.records, compressionRecord{
		strategy: block.Strategy,
		ratio:    block.Ratio,
		ts:       time.Now(),
	})

	// Keep a bounded window.
	if len(h.records) > s.cfg.BenchmarkSampleSize {
		h.records = h.records[len(h.records)-s.cfg.BenchmarkSampleSize:]
	}
}

// ShouldReEncode inspects the compression history for a feature and returns
// true with a recommended strategy if the current encoding is under-performing.
func (s *Selector) ShouldReEncode(feature string) (bool, Strategy) {
	if !s.cfg.EnableAdaptive {
		return false, StrategyNone
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	h, ok := s.history[feature]
	if !ok || len(h.records) < 2 {
		return false, StrategyNone
	}

	// Compare the average ratio of the last quarter to the overall average.
	n := len(h.records)
	quarter := n / 4
	if quarter < 1 {
		quarter = 1
	}

	var totalAvg, recentAvg float64
	for _, r := range h.records {
		totalAvg += r.ratio
	}
	totalAvg /= float64(n)

	for _, r := range h.records[n-quarter:] {
		recentAvg += r.ratio
	}
	recentAvg /= float64(quarter)

	// If recent performance dropped below the threshold, recommend re-encoding.
	if totalAvg == 0 || (totalAvg-recentAvg)/totalAvg < s.cfg.ReEncodingThreshold {
		return false, StrategyNone
	}

	// Find the strategy with the best historical ratio for this feature.
	best := make(map[Strategy]float64)
	counts := make(map[Strategy]int)
	for _, r := range h.records {
		best[r.strategy] += r.ratio
		counts[r.strategy]++
	}

	var bestStrat Strategy
	var bestRatio float64
	for st, sum := range best {
		avg := sum / float64(counts[st])
		if avg > bestRatio {
			bestRatio = avg
			bestStrat = st
		}
	}

	current := h.records[n-1].strategy
	if bestStrat == current {
		return false, StrategyNone
	}

	return true, bestStrat
}

// Stats returns aggregate compression metrics.
func (s *Selector) Stats() SelectorStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var avgRatio float64
	if s.ratioCount > 0 {
		avgRatio = s.ratioSum / float64(s.ratioCount)
	}

	dist := make(map[Strategy]int, len(s.stratDist))
	for k, v := range s.stratDist {
		dist[k] = v
	}

	return SelectorStats{
		TotalCompressed:      s.totalComp,
		TotalDecompressed:    s.totalDecomp,
		AvgRatio:             avgRatio,
		StrategyDistribution: dist,
	}
}

// ---------------------------------------------------------------------------
// Codec implementations (lightweight, self-contained)
// ---------------------------------------------------------------------------

// compressDictionary builds a byte-level dictionary and encodes indices.
func compressDictionary(data []byte) ([]byte, error) {
	dict := make(map[byte]uint8)
	var order []byte
	for _, b := range data {
		if _, ok := dict[b]; !ok {
			dict[b] = uint8(len(order))
			order = append(order, b)
		}
	}

	var buf bytes.Buffer
	// Header: dict size (1 byte), dict entries, then indices.
	buf.WriteByte(uint8(len(order)))
	buf.Write(order)
	for _, b := range data {
		buf.WriteByte(dict[b])
	}
	return buf.Bytes(), nil
}

func decompressDictionary(data []byte) ([]byte, error) {
	if len(data) < 1 {
		return nil, errors.New("dictionary: truncated header")
	}
	dictSize := int(data[0])
	if len(data) < 1+dictSize {
		return nil, errors.New("dictionary: truncated dict")
	}
	dict := data[1 : 1+dictSize]
	indices := data[1+dictSize:]

	out := make([]byte, len(indices))
	for i, idx := range indices {
		if int(idx) >= dictSize {
			return nil, fmt.Errorf("dictionary: index %d out of range", idx)
		}
		out[i] = dict[idx]
	}
	return out, nil
}

// compressDelta stores a base value then successive deltas as varints.
func compressDelta(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("delta: empty data")
	}

	var buf bytes.Buffer
	// Store length prefix so we can restore exact size on decompress.
	lenBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(lenBuf, uint64(len(data)))
	buf.Write(lenBuf[:n])

	prev := byte(0)
	for _, b := range data {
		delta := int8(b - prev)
		n := binary.PutVarint(lenBuf, int64(delta))
		buf.Write(lenBuf[:n])
		prev = b
	}
	return buf.Bytes(), nil
}

func decompressDelta(data []byte) ([]byte, error) {
	r := bytes.NewReader(data)

	length, err := binary.ReadUvarint(r)
	if err != nil {
		return nil, fmt.Errorf("delta: reading length: %w", err)
	}

	out := make([]byte, 0, length)
	prev := byte(0)
	for i := uint64(0); i < length; i++ {
		delta, err := binary.ReadVarint(r)
		if err != nil {
			return nil, fmt.Errorf("delta: reading varint at %d: %w", i, err)
		}
		prev += byte(delta)
		out = append(out, prev)
	}
	return out, nil
}

// compressRLE encodes runs of identical bytes as (count, value) pairs.
func compressRLE(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("rle: empty data")
	}

	var buf bytes.Buffer
	cur := data[0]
	count := uint64(1)

	flush := func() {
		tmp := make([]byte, binary.MaxVarintLen64)
		n := binary.PutUvarint(tmp, count)
		buf.Write(tmp[:n])
		buf.WriteByte(cur)
	}

	for i := 1; i < len(data); i++ {
		if data[i] == cur {
			count++
			continue
		}
		flush()
		cur = data[i]
		count = 1
	}
	flush()
	return buf.Bytes(), nil
}

func decompressRLE(data []byte) ([]byte, error) {
	r := bytes.NewReader(data)
	var out []byte
	for r.Len() > 0 {
		count, err := binary.ReadUvarint(r)
		if err != nil {
			return nil, fmt.Errorf("rle: reading count: %w", err)
		}
		val, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("rle: reading value: %w", err)
		}
		for i := uint64(0); i < count; i++ {
			out = append(out, val)
		}
	}
	return out, nil
}

// compressXOR stores successive XOR differences (good for similar floats).
func compressXOR(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("xor: empty data")
	}

	var buf bytes.Buffer
	lenBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(lenBuf, uint64(len(data)))
	buf.Write(lenBuf[:n])

	prev := byte(0)
	for _, b := range data {
		buf.WriteByte(b ^ prev)
		prev = b
	}
	return buf.Bytes(), nil
}

func decompressXOR(data []byte) ([]byte, error) {
	r := bytes.NewReader(data)
	length, err := binary.ReadUvarint(r)
	if err != nil {
		return nil, fmt.Errorf("xor: reading length: %w", err)
	}

	out := make([]byte, 0, length)
	prev := byte(0)
	for i := uint64(0); i < length; i++ {
		x, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("xor: reading byte at %d: %w", i, err)
		}
		prev ^= x
		out = append(out, prev)
	}
	return out, nil
}

// compressLZ4 implements a minimal LZ4-style sliding-window compressor.
// Format: [original-size varint] [blocks...] where each block is either
// a literal run (0 tag + length + data) or a back-reference (1 tag + offset + length).
func compressLZ4(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("lz4: empty data")
	}

	var buf bytes.Buffer
	tmp := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(tmp, uint64(len(data)))
	buf.Write(tmp[:n])

	const windowSize = 4096
	const minMatch = 4

	i := 0
	for i < len(data) {
		bestLen := 0
		bestOff := 0

		start := i - windowSize
		if start < 0 {
			start = 0
		}

		if i+minMatch <= len(data) {
			for j := start; j < i; j++ {
				ml := 0
				for i+ml < len(data) && ml < 255 && data[j+ml] == data[i+ml] {
					ml++
				}
				if ml >= minMatch && ml > bestLen {
					bestLen = ml
					bestOff = i - j
				}
			}
		}

		if bestLen >= minMatch {
			buf.WriteByte(1) // back-reference tag
			n := binary.PutUvarint(tmp, uint64(bestOff))
			buf.Write(tmp[:n])
			n = binary.PutUvarint(tmp, uint64(bestLen))
			buf.Write(tmp[:n])
			i += bestLen
		} else {
			// Gather a literal run up to the next match or end.
			litStart := i
			i++
			for i < len(data) {
				if i+minMatch > len(data) {
					i++
					continue
				}
				found := false
				s := i - windowSize
				if s < 0 {
					s = 0
				}
				for j := s; j < i; j++ {
					ml := 0
					for i+ml < len(data) && ml < 255 && data[j+ml] == data[i+ml] {
						ml++
					}
					if ml >= minMatch {
						found = true
						break
					}
				}
				if found {
					break
				}
				i++
			}
			buf.WriteByte(0) // literal tag
			litLen := i - litStart
			n := binary.PutUvarint(tmp, uint64(litLen))
			buf.Write(tmp[:n])
			buf.Write(data[litStart : litStart+litLen])
		}
	}

	return buf.Bytes(), nil
}

func decompressLZ4(data []byte, originalSize int64) ([]byte, error) {
	r := bytes.NewReader(data)
	origLen, err := binary.ReadUvarint(r)
	if err != nil {
		return nil, fmt.Errorf("lz4: reading length: %w", err)
	}

	out := make([]byte, 0, origLen)

	for r.Len() > 0 {
		tag, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("lz4: reading tag: %w", err)
		}

		switch tag {
		case 0: // literal
			litLen, err := binary.ReadUvarint(r)
			if err != nil {
				return nil, fmt.Errorf("lz4: reading literal length: %w", err)
			}
			lit := make([]byte, litLen)
			if _, err := r.Read(lit); err != nil {
				return nil, fmt.Errorf("lz4: reading literal data: %w", err)
			}
			out = append(out, lit...)
		case 1: // back-reference
			off, err := binary.ReadUvarint(r)
			if err != nil {
				return nil, fmt.Errorf("lz4: reading offset: %w", err)
			}
			ml, err := binary.ReadUvarint(r)
			if err != nil {
				return nil, fmt.Errorf("lz4: reading match length: %w", err)
			}
			start := len(out) - int(off)
			if start < 0 || start >= len(out) {
				return nil, fmt.Errorf("lz4: invalid back-reference offset %d", off)
			}
			for j := 0; j < int(ml); j++ {
				out = append(out, out[start+j])
			}
		default:
			return nil, fmt.Errorf("lz4: unknown tag %d", tag)
		}
	}

	return out, nil
}

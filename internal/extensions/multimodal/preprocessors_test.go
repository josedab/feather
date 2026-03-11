package multimodal

import (
	"strings"
	"testing"
)

func TestImagePreprocessor_Process(t *testing.T) {
	proc := NewImagePreprocessor(DefaultImageConfig())

	data := []byte("fake image data for testing purposes")
	result, err := proc.Process(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.InputType != PreprocessorImage {
		t.Errorf("expected input type %s, got %s", PreprocessorImage, result.InputType)
	}
	if result.InputSize != int64(len(data)) {
		t.Errorf("expected input size %d, got %d", len(data), result.InputSize)
	}
	if result.OutputSize != int64(224*224*3) {
		t.Errorf("expected output size %d, got %d", 224*224*3, result.OutputSize)
	}
	if !strings.HasPrefix(result.ID, "img-") {
		t.Errorf("expected ID prefix 'img-', got %s", result.ID)
	}
	if result.Metadata["format"] != "tensor" {
		t.Errorf("expected format 'tensor', got %s", result.Metadata["format"])
	}
	if result.Features["pixel_count"] != 224*224*3 {
		t.Errorf("expected pixel_count %d, got %v", 224*224*3, result.Features["pixel_count"])
	}
}

func TestImagePreprocessor_EmptyData(t *testing.T) {
	proc := NewImagePreprocessor(DefaultImageConfig())
	_, err := proc.Process([]byte{})
	if err == nil {
		t.Fatal("expected error for empty data")
	}
	if !strings.Contains(err.Error(), "empty image data") {
		t.Errorf("expected 'empty image data' error, got: %v", err)
	}
}

func TestImagePreprocessor_CustomConfig(t *testing.T) {
	config := ImageConfig{
		TargetWidth:  128,
		TargetHeight: 128,
		Normalize:    false,
		Augment:      true,
		Channels:     1,
	}
	proc := NewImagePreprocessor(config)
	result, err := proc.Process([]byte("test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.OutputSize != int64(128*128*1) {
		t.Errorf("expected output size %d, got %d", 128*128, result.OutputSize)
	}
	if result.Features["augmented"] != true {
		t.Error("expected augmented=true")
	}
}

func TestAudioPreprocessor_Process(t *testing.T) {
	proc := NewAudioPreprocessor(DefaultAudioConfig())

	// Create enough data for a meaningful duration (16-bit mono, 16kHz)
	data := make([]byte, 32000) // 1 second of audio
	for i := range data {
		data[i] = byte(i % 256)
	}

	result, err := proc.Process(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.InputType != PreprocessorAudio {
		t.Errorf("expected input type %s, got %s", PreprocessorAudio, result.InputType)
	}
	if !strings.HasPrefix(result.ID, "audio-") {
		t.Errorf("expected ID prefix 'audio-', got %s", result.ID)
	}
	if result.Features["sample_rate"] != 16000 {
		t.Errorf("expected sample_rate 16000, got %v", result.Features["sample_rate"])
	}
	if result.Metadata["format"] != "mfcc+spectrogram" {
		t.Errorf("expected format 'mfcc+spectrogram', got %s", result.Metadata["format"])
	}
}

func TestAudioPreprocessor_DurationLimiting(t *testing.T) {
	config := DefaultAudioConfig()
	config.MaxDurationSec = 5.0
	proc := NewAudioPreprocessor(config)

	// Create data that would represent > 5 seconds (16-bit mono, 16kHz)
	data := make([]byte, 320000) // ~10 seconds
	result, err := proc.Process(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dur, ok := result.Features["duration_sec"].(float64)
	if !ok {
		t.Fatal("duration_sec not found in features")
	}
	if dur > 5.0 {
		t.Errorf("expected duration <= 5.0, got %f", dur)
	}
}

func TestAudioPreprocessor_EmptyData(t *testing.T) {
	proc := NewAudioPreprocessor(DefaultAudioConfig())
	_, err := proc.Process([]byte{})
	if err == nil {
		t.Fatal("expected error for empty data")
	}
}

func TestTextPreprocessor_Process(t *testing.T) {
	proc := NewTextPreprocessor(DefaultTextConfig())

	data := []byte("Hello World this is a test of text preprocessing")
	result, err := proc.Process(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.InputType != PreprocessorText {
		t.Errorf("expected input type %s, got %s", PreprocessorText, result.InputType)
	}
	if !strings.HasPrefix(result.ID, "text-") {
		t.Errorf("expected ID prefix 'text-', got %s", result.ID)
	}

	totalTokens, ok := result.Features["total_tokens"].(int)
	if !ok {
		t.Fatal("total_tokens not found in features")
	}
	if totalTokens != 9 {
		t.Errorf("expected 9 tokens, got %d", totalTokens)
	}
}

func TestTextPreprocessor_HTMLStripping(t *testing.T) {
	config := DefaultTextConfig()
	proc := NewTextPreprocessor(config)

	data := []byte("<p>Hello <b>World</b></p>")
	result, err := proc.Process(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	totalTokens, ok := result.Features["total_tokens"].(int)
	if !ok {
		t.Fatal("total_tokens not found")
	}
	if totalTokens != 2 {
		t.Errorf("expected 2 tokens after HTML strip, got %d", totalTokens)
	}
}

func TestTextPreprocessor_Chunking(t *testing.T) {
	config := TextConfig{
		MaxTokens:    0,
		ChunkSize:    3,
		ChunkOverlap: 1,
		Lowercase:    false,
		StripHTML:    false,
	}
	proc := NewTextPreprocessor(config)

	data := []byte("one two three four five six seven")
	result, err := proc.Process(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	totalChunks, ok := result.Features["total_chunks"].(int)
	if !ok {
		t.Fatal("total_chunks not found")
	}
	if totalChunks < 2 {
		t.Errorf("expected at least 2 chunks, got %d", totalChunks)
	}
}

func TestTextPreprocessor_EmptyData(t *testing.T) {
	proc := NewTextPreprocessor(DefaultTextConfig())
	_, err := proc.Process([]byte{})
	if err == nil {
		t.Fatal("expected error for empty data")
	}
}

func TestTextPreprocessor_MaxTokens(t *testing.T) {
	config := TextConfig{
		MaxTokens:    3,
		ChunkSize:    256,
		ChunkOverlap: 0,
		Lowercase:    false,
		StripHTML:    false,
	}
	proc := NewTextPreprocessor(config)

	data := []byte("one two three four five")
	result, err := proc.Process(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	totalTokens, ok := result.Features["total_tokens"].(int)
	if !ok {
		t.Fatal("total_tokens not found")
	}
	if totalTokens != 3 {
		t.Errorf("expected 3 tokens (truncated), got %d", totalTokens)
	}
}

func TestStripHTMLTags(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"<p>hello</p>", "hello"},
		{"no tags", "no tags"},
		{"<b>bold</b> and <i>italic</i>", "bold and italic"},
		{"<div><span>nested</span></div>", "nested"},
		{"", ""},
	}
	for _, tt := range tests {
		got := stripHTMLTags(tt.input)
		if got != tt.expected {
			t.Errorf("stripHTMLTags(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

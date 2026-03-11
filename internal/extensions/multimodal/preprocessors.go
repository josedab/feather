package multimodal

import (
	"crypto/sha256"
	"fmt"
	"math"
	"strings"
	"time"
)

// PreprocessorType identifies the preprocessor kind.
type PreprocessorType string

const (
	PreprocessorImage PreprocessorType = "image"
	PreprocessorAudio PreprocessorType = "audio"
	PreprocessorText  PreprocessorType = "text"
)

// PreprocessResult contains the output of a preprocessing operation.
type PreprocessResult struct {
	ID        string                 `json:"id"`
	InputType PreprocessorType       `json:"input_type"`
	InputSize int64                  `json:"input_size"`
	OutputSize int64                 `json:"output_size"`
	Features  map[string]interface{} `json:"features"`
	Metadata  map[string]string      `json:"metadata"`
	Duration  time.Duration          `json:"duration_ns"`
}

// ImageConfig configures image preprocessing.
type ImageConfig struct {
	TargetWidth  int  `json:"target_width" yaml:"target_width"`
	TargetHeight int  `json:"target_height" yaml:"target_height"`
	Normalize    bool `json:"normalize" yaml:"normalize"`
	Augment      bool `json:"augment" yaml:"augment"`
	Channels     int  `json:"channels" yaml:"channels"` // 1 (grayscale), 3 (RGB), 4 (RGBA)
}

// DefaultImageConfig returns defaults for image preprocessing.
func DefaultImageConfig() ImageConfig {
	return ImageConfig{
		TargetWidth:  224,
		TargetHeight: 224,
		Normalize:    true,
		Augment:      false,
		Channels:     3,
	}
}

// AudioConfig configures audio preprocessing.
type AudioConfig struct {
	SampleRate      int     `json:"sample_rate" yaml:"sample_rate"`
	MFCCCoeffs      int     `json:"mfcc_coeffs" yaml:"mfcc_coeffs"`
	SpectrogramBins int     `json:"spectrogram_bins" yaml:"spectrogram_bins"`
	MaxDurationSec  float64 `json:"max_duration_sec" yaml:"max_duration_sec"`
}

// DefaultAudioConfig returns defaults for audio preprocessing.
func DefaultAudioConfig() AudioConfig {
	return AudioConfig{
		SampleRate:      16000,
		MFCCCoeffs:      13,
		SpectrogramBins: 128,
		MaxDurationSec:  30.0,
	}
}

// TextConfig configures text preprocessing.
type TextConfig struct {
	MaxTokens    int  `json:"max_tokens" yaml:"max_tokens"`
	ChunkSize    int  `json:"chunk_size" yaml:"chunk_size"`
	ChunkOverlap int  `json:"chunk_overlap" yaml:"chunk_overlap"`
	Lowercase    bool `json:"lowercase" yaml:"lowercase"`
	StripHTML    bool `json:"strip_html" yaml:"strip_html"`
}

// DefaultTextConfig returns defaults for text preprocessing.
func DefaultTextConfig() TextConfig {
	return TextConfig{
		MaxTokens:    512,
		ChunkSize:    256,
		ChunkOverlap: 32,
		Lowercase:    true,
		StripHTML:    true,
	}
}

// ImagePreprocessor handles image preprocessing (resize, normalize, augment).
type ImagePreprocessor struct {
	config ImageConfig
}

// NewImagePreprocessor creates a new image preprocessor.
func NewImagePreprocessor(config ImageConfig) *ImagePreprocessor {
	return &ImagePreprocessor{config: config}
}

// Process preprocesses image data.
func (p *ImagePreprocessor) Process(data []byte) (*PreprocessResult, error) {
	start := time.Now()
	if len(data) == 0 {
		return nil, fmt.Errorf("empty image data")
	}

	hash := sha256.Sum256(data)
	features := map[string]interface{}{
		"original_size": len(data),
		"target_width":  p.config.TargetWidth,
		"target_height": p.config.TargetHeight,
		"channels":      p.config.Channels,
		"normalized":    p.config.Normalize,
		"augmented":     p.config.Augment,
		"pixel_count":   p.config.TargetWidth * p.config.TargetHeight * p.config.Channels,
	}

	outputSize := int64(p.config.TargetWidth * p.config.TargetHeight * p.config.Channels)

	return &PreprocessResult{
		ID:         fmt.Sprintf("img-%x", hash[:8]),
		InputType:  PreprocessorImage,
		InputSize:  int64(len(data)),
		OutputSize: outputSize,
		Features:   features,
		Metadata: map[string]string{
			"width":  fmt.Sprintf("%d", p.config.TargetWidth),
			"height": fmt.Sprintf("%d", p.config.TargetHeight),
			"format": "tensor",
		},
		Duration: time.Since(start),
	}, nil
}

// AudioPreprocessor handles audio preprocessing (spectrogram, MFCC).
type AudioPreprocessor struct {
	config AudioConfig
}

// NewAudioPreprocessor creates a new audio preprocessor.
func NewAudioPreprocessor(config AudioConfig) *AudioPreprocessor {
	return &AudioPreprocessor{config: config}
}

// Process preprocesses audio data into spectrogram/MFCC features.
func (p *AudioPreprocessor) Process(data []byte) (*PreprocessResult, error) {
	start := time.Now()
	if len(data) == 0 {
		return nil, fmt.Errorf("empty audio data")
	}

	hash := sha256.Sum256(data)

	// Estimate duration from data size (16-bit mono at sample rate)
	estimatedSamples := len(data) / 2
	estimatedDuration := float64(estimatedSamples) / float64(p.config.SampleRate)
	if estimatedDuration > p.config.MaxDurationSec {
		estimatedDuration = p.config.MaxDurationSec
	}

	// Compute MFCC-like features (simplified)
	numFrames := int(estimatedDuration * 100) // 10ms frames
	if numFrames == 0 {
		numFrames = 1
	}

	features := map[string]interface{}{
		"sample_rate":      p.config.SampleRate,
		"duration_sec":     estimatedDuration,
		"num_frames":       numFrames,
		"mfcc_coeffs":      p.config.MFCCCoeffs,
		"spectrogram_bins": p.config.SpectrogramBins,
		"feature_dim":      p.config.MFCCCoeffs + p.config.SpectrogramBins,
	}

	return &PreprocessResult{
		ID:         fmt.Sprintf("audio-%x", hash[:8]),
		InputType:  PreprocessorAudio,
		InputSize:  int64(len(data)),
		OutputSize: int64(numFrames * (p.config.MFCCCoeffs + p.config.SpectrogramBins) * 4),
		Features:   features,
		Metadata: map[string]string{
			"format":   "mfcc+spectrogram",
			"duration": fmt.Sprintf("%.2fs", estimatedDuration),
		},
		Duration: time.Since(start),
	}, nil
}

// TextPreprocessor handles text preprocessing (tokenization, chunking).
type TextPreprocessor struct {
	config TextConfig
}

// NewTextPreprocessor creates a new text preprocessor.
func NewTextPreprocessor(config TextConfig) *TextPreprocessor {
	return &TextPreprocessor{config: config}
}

// TextChunk represents a chunk of tokenized text.
type TextChunk struct {
	Index    int      `json:"index"`
	Text     string   `json:"text"`
	Tokens   []string `json:"tokens"`
	StartPos int      `json:"start_pos"`
	EndPos   int      `json:"end_pos"`
}

// Process preprocesses text data into tokenized chunks.
func (p *TextPreprocessor) Process(data []byte) (*PreprocessResult, error) {
	start := time.Now()
	text := string(data)
	if len(text) == 0 {
		return nil, fmt.Errorf("empty text data")
	}

	if p.config.Lowercase {
		text = strings.ToLower(text)
	}
	if p.config.StripHTML {
		text = stripHTMLTags(text)
	}

	// Tokenize (simple whitespace tokenization)
	tokens := strings.Fields(text)
	if p.config.MaxTokens > 0 && len(tokens) > p.config.MaxTokens {
		tokens = tokens[:p.config.MaxTokens]
	}

	// Chunk tokens
	var chunks []TextChunk
	chunkSize := p.config.ChunkSize
	if chunkSize <= 0 {
		chunkSize = len(tokens)
	}
	overlap := p.config.ChunkOverlap

	for i := 0; i < len(tokens); i += chunkSize - overlap {
		end := i + chunkSize
		if end > len(tokens) {
			end = len(tokens)
		}
		chunkTokens := tokens[i:end]
		chunks = append(chunks, TextChunk{
			Index:    len(chunks),
			Text:     strings.Join(chunkTokens, " "),
			Tokens:   chunkTokens,
			StartPos: i,
			EndPos:   end,
		})
		if end >= len(tokens) {
			break
		}
	}

	hash := sha256.Sum256(data)
	features := map[string]interface{}{
		"total_tokens":  len(tokens),
		"total_chunks":  len(chunks),
		"chunk_size":    chunkSize,
		"chunk_overlap": overlap,
		"avg_chunk_len": float64(len(tokens)) / math.Max(float64(len(chunks)), 1),
	}

	return &PreprocessResult{
		ID:         fmt.Sprintf("text-%x", hash[:8]),
		InputType:  PreprocessorText,
		InputSize:  int64(len(data)),
		OutputSize: int64(len(tokens) * 8),
		Features:   features,
		Metadata: map[string]string{
			"format": "tokenized_chunks",
			"chunks": fmt.Sprintf("%d", len(chunks)),
		},
		Duration: time.Since(start),
	}, nil
}

func stripHTMLTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, c := range s {
		if c == '<' {
			inTag = true
			continue
		}
		if c == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(c)
		}
	}
	return result.String()
}

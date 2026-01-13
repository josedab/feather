package llm

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ChunkMethod defines the text chunking strategy.
type ChunkMethod string

const (
	// ChunkMethodFixed splits text into fixed-size chunks.
	ChunkMethodFixed ChunkMethod = "fixed"
	// ChunkMethodSemantic splits at sentence/paragraph boundaries.
	ChunkMethodSemantic ChunkMethod = "semantic"
	// ChunkMethodRecursive uses hierarchical splitting with configurable separators.
	ChunkMethodRecursive ChunkMethod = "recursive"
	// ChunkMethodSentence splits strictly at sentence boundaries.
	ChunkMethodSentence ChunkMethod = "sentence"
	// ChunkMethodParagraph splits at paragraph boundaries.
	ChunkMethodParagraph ChunkMethod = "paragraph"
)

// ChunkerConfig configures the text chunker.
type ChunkerConfig struct {
	// Method is the chunking strategy.
	Method ChunkMethod `json:"method" yaml:"method"`
	// ChunkSize is the target chunk size in characters.
	ChunkSize int `json:"chunk_size" yaml:"chunk_size"`
	// ChunkOverlap is the overlap between consecutive chunks.
	ChunkOverlap int `json:"chunk_overlap" yaml:"chunk_overlap"`
	// Separators for recursive chunking (ordered by priority).
	Separators []string `json:"separators,omitempty" yaml:"separators,omitempty"`
	// MinChunkSize is the minimum chunk size (smaller chunks are merged).
	MinChunkSize int `json:"min_chunk_size" yaml:"min_chunk_size"`
	// PreserveSentences tries to keep sentences intact.
	PreserveSentences bool `json:"preserve_sentences" yaml:"preserve_sentences"`
}

// DefaultChunkerConfig returns the default chunker configuration.
func DefaultChunkerConfig() ChunkerConfig {
	return ChunkerConfig{
		Method:            ChunkMethodSemantic,
		ChunkSize:         512,
		ChunkOverlap:      50,
		MinChunkSize:      100,
		PreserveSentences: true,
		Separators:        []string{"\n\n", "\n", ". ", "! ", "? ", "; ", ", ", " "},
	}
}

// Chunker splits text into chunks for embedding.
type Chunker struct {
	config ChunkerConfig
}

// Chunk represents a piece of text with position info.
type Chunk struct {
	// Text is the chunk content.
	Text string `json:"text"`
	// Index is the chunk index (0-based).
	Index int `json:"index"`
	// StartChar is the starting character position in original text.
	StartChar int `json:"start_char"`
	// EndChar is the ending character position in original text.
	EndChar int `json:"end_char"`
	// Metadata contains additional info about the chunk.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// NewChunker creates a new text chunker.
func NewChunker(config ChunkerConfig) *Chunker {
	if config.ChunkSize == 0 {
		config.ChunkSize = 512
	}
	if config.MinChunkSize == 0 {
		config.MinChunkSize = 100
	}
	if len(config.Separators) == 0 {
		config.Separators = []string{"\n\n", "\n", ". ", "! ", "? ", "; ", ", ", " "}
	}

	return &Chunker{config: config}
}

// Split divides text into chunks.
func (c *Chunker) Split(text string) []Chunk {
	if len(text) == 0 {
		return nil
	}

	// Normalize whitespace
	text = normalizeWhitespace(text)

	switch c.config.Method {
	case ChunkMethodFixed:
		return c.splitFixed(text)
	case ChunkMethodSemantic:
		return c.splitSemantic(text)
	case ChunkMethodRecursive:
		return c.splitRecursive(text, c.config.Separators)
	case ChunkMethodSentence:
		return c.splitSentence(text)
	case ChunkMethodParagraph:
		return c.splitParagraph(text)
	default:
		return c.splitSemantic(text)
	}
}

func (c *Chunker) splitFixed(text string) []Chunk {
	var chunks []Chunk
	runes := []rune(text)
	length := len(runes)

	step := c.config.ChunkSize - c.config.ChunkOverlap
	if step <= 0 {
		step = c.config.ChunkSize
	}

	for start := 0; start < length; start += step {
		end := start + c.config.ChunkSize
		if end > length {
			end = length
		}

		chunkText := string(runes[start:end])
		if len(strings.TrimSpace(chunkText)) > 0 {
			chunks = append(chunks, Chunk{
				Text:      chunkText,
				Index:     len(chunks),
				StartChar: start,
				EndChar:   end,
			})
		}

		if end >= length {
			break
		}
	}

	return chunks
}

func (c *Chunker) splitSemantic(text string) []Chunk {
	// Split into sentences first
	sentences := splitSentences(text)

	var chunks []Chunk
	var current strings.Builder
	startChar := 0
	currentStart := 0

	for _, sentence := range sentences {
		sentLen := utf8.RuneCountInString(sentence)

		// If adding this sentence exceeds chunk size
		if current.Len() > 0 && utf8.RuneCountInString(current.String())+sentLen > c.config.ChunkSize {
			// Save current chunk
			chunkText := strings.TrimSpace(current.String())
			if len(chunkText) >= c.config.MinChunkSize {
				chunks = append(chunks, Chunk{
					Text:      chunkText,
					Index:     len(chunks),
					StartChar: currentStart,
					EndChar:   startChar,
				})
			}

			// Handle overlap
			if c.config.ChunkOverlap > 0 {
				current.Reset()
				overlapText := getOverlapText(chunkText, c.config.ChunkOverlap)
				current.WriteString(overlapText)
				currentStart = startChar - utf8.RuneCountInString(overlapText)
			} else {
				current.Reset()
				currentStart = startChar
			}
		}

		current.WriteString(sentence)
		startChar += sentLen
	}

	// Add remaining text
	if current.Len() > 0 {
		chunkText := strings.TrimSpace(current.String())
		if len(chunkText) > 0 {
			chunks = append(chunks, Chunk{
				Text:      chunkText,
				Index:     len(chunks),
				StartChar: currentStart,
				EndChar:   startChar,
			})
		}
	}

	return c.mergeSmallChunks(chunks)
}

func (c *Chunker) splitRecursive(text string, separators []string) []Chunk {
	if len(separators) == 0 {
		return c.splitFixed(text)
	}

	// Try to split with current separator
	sep := separators[0]
	parts := strings.Split(text, sep)

	if len(parts) == 1 {
		// No split occurred, try next separator
		return c.splitRecursive(text, separators[1:])
	}

	var chunks []Chunk
	var current strings.Builder
	startChar := 0
	currentStart := 0

	for i, part := range parts {
		partLen := utf8.RuneCountInString(part)
		sepLen := utf8.RuneCountInString(sep)

		// If this part alone is too large, recursively split it
		if partLen > c.config.ChunkSize {
			// Save current chunk first
			if current.Len() > 0 {
				chunkText := strings.TrimSpace(current.String())
				if len(chunkText) >= c.config.MinChunkSize {
					chunks = append(chunks, Chunk{
						Text:      chunkText,
						Index:     len(chunks),
						StartChar: currentStart,
						EndChar:   startChar,
					})
				}
				current.Reset()
			}

			// Recursively split the large part
			subChunks := c.splitRecursive(part, separators[1:])
			for _, sc := range subChunks {
				sc.Index = len(chunks)
				sc.StartChar += startChar
				sc.EndChar += startChar
				chunks = append(chunks, sc)
			}
			startChar += partLen
			if i < len(parts)-1 {
				startChar += sepLen
			}
			currentStart = startChar
			continue
		}

		// If adding this part exceeds chunk size
		if current.Len() > 0 && utf8.RuneCountInString(current.String())+partLen+sepLen > c.config.ChunkSize {
			chunkText := strings.TrimSpace(current.String())
			if len(chunkText) >= c.config.MinChunkSize {
				chunks = append(chunks, Chunk{
					Text:      chunkText,
					Index:     len(chunks),
					StartChar: currentStart,
					EndChar:   startChar,
				})
			}
			current.Reset()
			currentStart = startChar
		}

		current.WriteString(part)
		if i < len(parts)-1 {
			current.WriteString(sep)
		}
		startChar += partLen
		if i < len(parts)-1 {
			startChar += sepLen
		}
	}

	// Add remaining
	if current.Len() > 0 {
		chunkText := strings.TrimSpace(current.String())
		if len(chunkText) > 0 {
			chunks = append(chunks, Chunk{
				Text:      chunkText,
				Index:     len(chunks),
				StartChar: currentStart,
				EndChar:   startChar,
			})
		}
	}

	return c.mergeSmallChunks(chunks)
}

func (c *Chunker) splitSentence(text string) []Chunk {
	sentences := splitSentences(text)
	chunks := make([]Chunk, 0, len(sentences))
	startChar := 0

	for i, sent := range sentences {
		sent = strings.TrimSpace(sent)
		if len(sent) == 0 {
			continue
		}
		chunks = append(chunks, Chunk{
			Text:      sent,
			Index:     i,
			StartChar: startChar,
			EndChar:   startChar + utf8.RuneCountInString(sent),
		})
		startChar += utf8.RuneCountInString(sent) + 1
	}

	return chunks
}

func (c *Chunker) splitParagraph(text string) []Chunk {
	paragraphs := strings.Split(text, "\n\n")
	chunks := make([]Chunk, 0, len(paragraphs))
	startChar := 0

	for i, para := range paragraphs {
		para = strings.TrimSpace(para)
		if len(para) == 0 {
			startChar += 2
			continue
		}

		// If paragraph is too long, split it further
		if utf8.RuneCountInString(para) > c.config.ChunkSize {
			subChunker := NewChunker(ChunkerConfig{
				Method:       ChunkMethodSemantic,
				ChunkSize:    c.config.ChunkSize,
				ChunkOverlap: c.config.ChunkOverlap,
				MinChunkSize: c.config.MinChunkSize,
			})
			subChunks := subChunker.Split(para)
			for _, sc := range subChunks {
				sc.Index = len(chunks)
				sc.StartChar += startChar
				sc.EndChar += startChar
				chunks = append(chunks, sc)
			}
		} else {
			chunks = append(chunks, Chunk{
				Text:      para,
				Index:     i,
				StartChar: startChar,
				EndChar:   startChar + utf8.RuneCountInString(para),
			})
		}
		startChar += utf8.RuneCountInString(para) + 2
	}

	return chunks
}

func (c *Chunker) mergeSmallChunks(chunks []Chunk) []Chunk {
	if len(chunks) <= 1 || c.config.MinChunkSize == 0 {
		return chunks
	}

	var merged []Chunk
	var current *Chunk

	for i := range chunks {
		chunk := &chunks[i]
		if utf8.RuneCountInString(chunk.Text) < c.config.MinChunkSize {
			if current == nil {
				current = &Chunk{
					Text:      chunk.Text,
					Index:     len(merged),
					StartChar: chunk.StartChar,
					EndChar:   chunk.EndChar,
				}
			} else {
				current.Text += " " + chunk.Text
				current.EndChar = chunk.EndChar
			}
		} else {
			if current != nil {
				// Merge small chunk with current
				current.Text += " " + chunk.Text
				current.EndChar = chunk.EndChar
				merged = append(merged, *current)
				current = nil
			} else {
				chunk.Index = len(merged)
				merged = append(merged, *chunk)
			}
		}
	}

	if current != nil {
		if len(merged) > 0 {
			// Merge with last chunk
			merged[len(merged)-1].Text += " " + current.Text
			merged[len(merged)-1].EndChar = current.EndChar
		} else {
			merged = append(merged, *current)
		}
	}

	return merged
}

// Helper functions

var sentenceEndRegex = regexp.MustCompile(`[.!?]+[\s]+`)

func splitSentences(text string) []string {
	indices := sentenceEndRegex.FindAllStringIndex(text, -1)
	if len(indices) == 0 {
		return []string{text}
	}

	sentences := make([]string, 0, len(indices)+1)
	start := 0
	for _, idx := range indices {
		end := idx[1]
		sentences = append(sentences, text[start:end])
		start = end
	}
	if start < len(text) {
		sentences = append(sentences, text[start:])
	}

	return sentences
}

func normalizeWhitespace(text string) string {
	var result strings.Builder
	prevSpace := false

	for _, r := range text {
		if unicode.IsSpace(r) {
			if r == '\n' {
				// Preserve newlines
				if prevSpace && result.Len() > 0 {
					// Remove trailing space before newline
					s := result.String()
					if s[len(s)-1] == ' ' {
						result.Reset()
						result.WriteString(s[:len(s)-1])
					}
				}
				result.WriteRune('\n')
				prevSpace = true
			} else if !prevSpace {
				result.WriteRune(' ')
				prevSpace = true
			}
		} else {
			result.WriteRune(r)
			prevSpace = false
		}
	}

	return strings.TrimSpace(result.String())
}

func getOverlapText(text string, overlapSize int) string {
	runes := []rune(text)
	if len(runes) <= overlapSize {
		return text
	}

	start := len(runes) - overlapSize
	// Try to start at a word boundary
	for i := start; i < len(runes); i++ {
		if unicode.IsSpace(runes[i]) {
			return string(runes[i+1:])
		}
	}
	return string(runes[start:])
}

// EstimateTokens provides a rough token count estimate.
func EstimateTokens(text string) int {
	// Rough estimate: ~4 characters per token for English
	return (len(text) + 3) / 4
}

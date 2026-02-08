package rag

import (
	"strings"

	"github.com/google/uuid"
)

// ChunkStrategy defines the document chunking approach.
type ChunkStrategy string

const (
	// ChunkBySize splits text into fixed-size chunks with optional overlap.
	ChunkBySize ChunkStrategy = "fixed_size"
	// ChunkBySentence splits text at sentence boundaries.
	ChunkBySentence ChunkStrategy = "sentence"
	// ChunkByParagraph splits text at paragraph boundaries.
	ChunkByParagraph ChunkStrategy = "paragraph"
	// ChunkBySemantic splits text using semantic heuristics.
	ChunkBySemantic ChunkStrategy = "semantic"
)

// Chunker splits document content into chunks using a configured strategy.
type Chunker struct {
	strategy  ChunkStrategy
	chunkSize int
	overlap   int
}

// NewChunker creates a new Chunker with the given strategy, target size, and overlap.
func NewChunker(strategy ChunkStrategy, size, overlap int) *Chunker {
	if size <= 0 {
		size = 512
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= size {
		overlap = 0
	}
	return &Chunker{
		strategy:  strategy,
		chunkSize: size,
		overlap:   overlap,
	}
}

// Chunk splits content into a slice of Chunk using the configured strategy.
func (c *Chunker) Chunk(content string) []*Chunk {
	if len(strings.TrimSpace(content)) == 0 {
		return nil
	}

	switch c.strategy {
	case ChunkBySentence:
		return c.chunkBySentence(content)
	case ChunkByParagraph:
		return c.chunkByParagraph(content)
	case ChunkBySemantic:
		// Semantic falls back to sentence-based for the local implementation.
		return c.chunkBySentence(content)
	default:
		return c.chunkBySize(content)
	}
}

// chunkBySize splits content into fixed-size chunks with overlap.
func (c *Chunker) chunkBySize(content string) []*Chunk {
	runes := []rune(content)
	length := len(runes)
	if length == 0 {
		return nil
	}

	step := c.chunkSize - c.overlap
	if step <= 0 {
		step = c.chunkSize
	}

	var chunks []*Chunk
	for start := 0; start < length; start += step {
		end := start + c.chunkSize
		if end > length {
			end = length
		}

		text := string(runes[start:end])
		if len(strings.TrimSpace(text)) == 0 {
			continue
		}

		chunks = append(chunks, &Chunk{
			ID:       uuid.New().String(),
			Content:  text,
			Index:    len(chunks),
			StartPos: start,
			EndPos:   end,
		})

		if end >= length {
			break
		}
	}
	return chunks
}

// chunkBySentence splits content at sentence boundaries (., !, ?).
func (c *Chunker) chunkBySentence(content string) []*Chunk {
	sentences := splitOnSentences(content)
	if len(sentences) == 0 {
		return nil
	}

	var chunks []*Chunk
	var current strings.Builder
	startPos := 0
	currentStart := 0

	for _, sent := range sentences {
		sent = strings.TrimSpace(sent)
		if len(sent) == 0 {
			continue
		}

		// If adding this sentence would exceed chunk size and we have content, flush.
		if current.Len() > 0 && current.Len()+1+len(sent) > c.chunkSize {
			chunks = append(chunks, &Chunk{
				ID:       uuid.New().String(),
				Content:  current.String(),
				Index:    len(chunks),
				StartPos: currentStart,
				EndPos:   startPos,
			})
			current.Reset()
			currentStart = startPos
		}

		if current.Len() > 0 {
			current.WriteString(" ")
		}
		current.WriteString(sent)
		startPos += len(sent) + 1
	}

	if current.Len() > 0 {
		chunks = append(chunks, &Chunk{
			ID:       uuid.New().String(),
			Content:  current.String(),
			Index:    len(chunks),
			StartPos: currentStart,
			EndPos:   startPos,
		})
	}

	return chunks
}

// chunkByParagraph splits content at double-newline boundaries.
func (c *Chunker) chunkByParagraph(content string) []*Chunk {
	paragraphs := strings.Split(content, "\n\n")

	var chunks []*Chunk
	pos := 0
	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if len(para) == 0 {
			pos += 2
			continue
		}

		chunks = append(chunks, &Chunk{
			ID:       uuid.New().String(),
			Content:  para,
			Index:    len(chunks),
			StartPos: pos,
			EndPos:   pos + len(para),
		})
		pos += len(para) + 2
	}

	return chunks
}

// splitOnSentences splits text on sentence-ending punctuation followed by space.
func splitOnSentences(text string) []string {
	var sentences []string
	var current strings.Builder

	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		current.WriteRune(runes[i])

		if runes[i] == '.' || runes[i] == '!' || runes[i] == '?' {
			// Check if next char is a space or end of text.
			if i+1 >= len(runes) || runes[i+1] == ' ' || runes[i+1] == '\n' {
				s := strings.TrimSpace(current.String())
				if len(s) > 0 {
					sentences = append(sentences, s)
				}
				current.Reset()
				// Skip the space after punctuation.
				if i+1 < len(runes) && (runes[i+1] == ' ' || runes[i+1] == '\n') {
					i++
				}
			}
		}
	}

	// Remaining text.
	if s := strings.TrimSpace(current.String()); len(s) > 0 {
		sentences = append(sentences, s)
	}

	return sentences
}

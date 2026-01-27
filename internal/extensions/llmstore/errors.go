package llmstore

import "errors"

var (
	ErrPromptNotFound    = errors.New("prompt template not found")
	ErrPromptExists      = errors.New("prompt template already exists")
	ErrEmbeddingNotFound = errors.New("embedding not found")
	ErrPipelineNotFound  = errors.New("RAG pipeline not found")
	ErrPipelineExists    = errors.New("RAG pipeline already exists")
	ErrInvalidConfig     = errors.New("invalid configuration")
)

package multimodal

import (
	"fmt"
	"time"
)

// TransformType identifies a media transformation.
type TransformType string

const (
	TransformResize        TransformType = "resize"
	TransformCrop          TransformType = "crop"
	TransformThumbnail     TransformType = "thumbnail"
	TransformExtractFrames TransformType = "extract_frames"
	TransformNormalize     TransformType = "normalize"
	TransformAugment       TransformType = "augment"
)

// TransformConfig holds configuration for a single transform step.
type TransformConfig map[string]interface{}

// TransformResult holds the output of a transform pipeline execution.
type TransformResult struct {
	InputID    string            `json:"input_id"`
	OutputData []byte           `json:"output_data"`
	OutputType string            `json:"output_type"`
	Transform  TransformType     `json:"transform"`
	Duration   time.Duration     `json:"duration"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type transformStep struct {
	transform TransformType
	config    TransformConfig
}

// TransformPipeline represents a chain of media transformations.
type TransformPipeline struct {
	steps []transformStep
}

// NewTransformPipeline creates a new empty transform pipeline.
func NewTransformPipeline() *TransformPipeline {
	return &TransformPipeline{}
}

// AddStep adds a transformation step to the pipeline (fluent API).
func (p *TransformPipeline) AddStep(transform TransformType, config TransformConfig) *TransformPipeline {
	p.steps = append(p.steps, transformStep{transform: transform, config: config})
	return p
}

// Execute runs the pipeline on the input data.
func (p *TransformPipeline) Execute(inputData []byte, modality ModalityType) (*TransformResult, error) {
	if len(p.steps) == 0 {
		return nil, fmt.Errorf("pipeline has no steps")
	}

	start := time.Now()
	data := inputData
	var lastTransform TransformType

	for _, step := range p.steps {
		var err error
		data, err = applyTransform(data, modality, step)
		if err != nil {
			return nil, fmt.Errorf("applying %s: %w", step.transform, err)
		}
		lastTransform = step.transform
	}

	return &TransformResult{
		OutputData: data,
		OutputType: string(modality),
		Transform:  lastTransform,
		Duration:   time.Since(start),
		Metadata:   map[string]string{"steps": fmt.Sprintf("%d", len(p.steps))},
	}, nil
}

// Steps returns the list of transform types in the pipeline.
func (p *TransformPipeline) Steps() []TransformType {
	result := make([]TransformType, len(p.steps))
	for i, s := range p.steps {
		result[i] = s.transform
	}
	return result
}

// applyTransform applies a single transformation step.
// Actual image/video/audio transforms would require external libraries;
// this provides a pass-through framework.
func applyTransform(data []byte, modality ModalityType, step transformStep) ([]byte, error) {
	switch step.transform {
	case TransformNormalize:
		// Pass through — real normalization would depend on modality
		return data, nil
	case TransformResize, TransformCrop, TransformThumbnail:
		// Framework stub — would use image processing library
		return data, nil
	case TransformExtractFrames:
		if modality != ModalityVideo {
			return nil, fmt.Errorf("extract_frames requires video modality")
		}
		return data, nil
	case TransformAugment:
		return data, nil
	default:
		return nil, fmt.Errorf("unknown transform: %s", step.transform)
	}
}

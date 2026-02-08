// Package streamdsl provides a declarative DSL for defining streaming feature
// transformations that compile to an execution plan.
//
// Usage:
//
//	spec := streamdsl.PipelineSpec{
//	    Name:    "user-activity",
//	    Version: "1.0",
//	    Sources: []streamdsl.SourceSpec{{Name: "clicks", Type: "kafka"}},
//	    Sinks:   []streamdsl.SinkSpec{{Name: "output", Input: "clicks", Type: "feature_store"}},
//	}
//	pm := streamdsl.NewPipelineManager(streamdsl.DefaultCompilerConfig())
//	plan, err := pm.Compile(spec)
package streamdsl

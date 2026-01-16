package autofe

import (
	"fmt"
	"strings"
)

// CodeGenerator produces source code from candidate features.
type CodeGenerator struct{}

// NewCodeGenerator creates a new CodeGenerator.
func NewCodeGenerator() *CodeGenerator {
	return &CodeGenerator{}
}

// GenerateGo produces Go feature computation code from candidates.
func (g *CodeGenerator) GenerateGo(candidates []*CandidateFeature) string {
	var b strings.Builder
	b.WriteString("package features\n\nimport \"math\"\n\n")

	for _, c := range candidates {
		funcName := goFuncName(c.Name)
		b.WriteString(fmt.Sprintf("// %s computes %s.\n", funcName, c.Name))
		b.WriteString(fmt.Sprintf("// %s\n", c.Explanation))

		switch c.Transform {
		case TransformLog:
			b.WriteString(fmt.Sprintf("func %s(%s float64) float64 {\n\treturn math.Log(%s)\n}\n\n",
				funcName, c.SourceFeatures[0], c.SourceFeatures[0]))
		case TransformSqrt:
			b.WriteString(fmt.Sprintf("func %s(%s float64) float64 {\n\treturn math.Sqrt(%s)\n}\n\n",
				funcName, c.SourceFeatures[0], c.SourceFeatures[0]))
		case TransformSquare:
			b.WriteString(fmt.Sprintf("func %s(%s float64) float64 {\n\treturn %s * %s\n}\n\n",
				funcName, c.SourceFeatures[0], c.SourceFeatures[0], c.SourceFeatures[0]))
		case TransformInteraction:
			if len(c.SourceFeatures) >= 2 {
				b.WriteString(fmt.Sprintf("func %s(%s, %s float64) float64 {\n\treturn %s * %s\n}\n\n",
					funcName, c.SourceFeatures[0], c.SourceFeatures[1],
					c.SourceFeatures[0], c.SourceFeatures[1]))
			}
		case TransformRatio:
			if len(c.SourceFeatures) >= 2 {
				b.WriteString(fmt.Sprintf("func %s(%s, %s float64) float64 {\n\tif %s == 0 {\n\t\treturn 0\n\t}\n\treturn %s / %s\n}\n\n",
					funcName, c.SourceFeatures[0], c.SourceFeatures[1],
					c.SourceFeatures[1], c.SourceFeatures[0], c.SourceFeatures[1]))
			}
		default:
			b.WriteString(fmt.Sprintf("func %s(value float64) float64 {\n\t// %s\n\treturn value\n}\n\n",
				funcName, c.Expression))
		}
	}

	return b.String()
}

// GeneratePython produces Python feature computation code from candidates.
func (g *CodeGenerator) GeneratePython(candidates []*CandidateFeature) string {
	var b strings.Builder
	b.WriteString("import math\n\n")

	for _, c := range candidates {
		funcName := pyFuncName(c.Name)

		switch c.Transform {
		case TransformLog:
			b.WriteString(fmt.Sprintf("def %s(%s: float) -> float:\n    \"\"\"%s\"\"\"\n    return math.log(%s)\n\n",
				funcName, c.SourceFeatures[0], c.Explanation, c.SourceFeatures[0]))
		case TransformSqrt:
			b.WriteString(fmt.Sprintf("def %s(%s: float) -> float:\n    \"\"\"%s\"\"\"\n    return math.sqrt(%s)\n\n",
				funcName, c.SourceFeatures[0], c.Explanation, c.SourceFeatures[0]))
		case TransformSquare:
			b.WriteString(fmt.Sprintf("def %s(%s: float) -> float:\n    \"\"\"%s\"\"\"\n    return %s ** 2\n\n",
				funcName, c.SourceFeatures[0], c.Explanation, c.SourceFeatures[0]))
		case TransformInteraction:
			if len(c.SourceFeatures) >= 2 {
				b.WriteString(fmt.Sprintf("def %s(%s: float, %s: float) -> float:\n    \"\"\"%s\"\"\"\n    return %s * %s\n\n",
					funcName, c.SourceFeatures[0], c.SourceFeatures[1],
					c.Explanation, c.SourceFeatures[0], c.SourceFeatures[1]))
			}
		case TransformRatio:
			if len(c.SourceFeatures) >= 2 {
				b.WriteString(fmt.Sprintf("def %s(%s: float, %s: float) -> float:\n    \"\"\"%s\"\"\"\n    if %s == 0:\n        return 0.0\n    return %s / %s\n\n",
					funcName, c.SourceFeatures[0], c.SourceFeatures[1],
					c.Explanation, c.SourceFeatures[1], c.SourceFeatures[0], c.SourceFeatures[1]))
			}
		default:
			b.WriteString(fmt.Sprintf("def %s(value: float) -> float:\n    \"\"\"%s\"\"\"\n    # %s\n    return value\n\n",
				funcName, c.Explanation, c.Expression))
		}
	}

	return b.String()
}

// GenerateFeatherQL produces FeatherQL pipeline definitions from candidates.
func (g *CodeGenerator) GenerateFeatherQL(candidates []*CandidateFeature) string {
	var b strings.Builder
	b.WriteString("-- FeatherQL Pipeline: Auto-generated feature transforms\n\n")

	for _, c := range candidates {
		b.WriteString(fmt.Sprintf("CREATE FEATURE %s AS\n  %s\n", c.Name, c.Expression))
		if len(c.SourceFeatures) > 0 {
			b.WriteString(fmt.Sprintf("  FROM %s\n", strings.Join(c.SourceFeatures, ", ")))
		}
		b.WriteString(fmt.Sprintf("  -- score: %.2f, transform: %s\n;\n\n", c.Score, c.Transform))
	}

	return b.String()
}

func goFuncName(name string) string {
	parts := strings.Split(name, "_")
	var result strings.Builder
	result.WriteString("Compute")
	for _, p := range parts {
		if len(p) > 0 {
			result.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	return result.String()
}

func pyFuncName(name string) string {
	return "compute_" + strings.ReplaceAll(name, " ", "_")
}

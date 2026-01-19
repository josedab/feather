package pipelinebuilder

import (
	"fmt"
	"strings"
)

// CodeGenConfig controls code generation output.
type CodeGenConfig struct {
	Language        string `json:"language"` // "go", "python", "featherql"
	IncludeComments bool   `json:"include_comments"`
	PackageName     string `json:"package_name"`
}

// CodeGenerator generates executable code from a pipeline definition.
type CodeGenerator struct {
	config CodeGenConfig
}

// NewCodeGenerator creates a code generator with the given configuration.
func NewCodeGenerator(config CodeGenConfig) *CodeGenerator {
	return &CodeGenerator{config: config}
}

// Generate produces code for the given pipeline and its transforms.
func (g *CodeGenerator) Generate(pipeline *Pipeline, registry *TransformRegistry) (string, error) {
	order, err := pipeline.TopologicalSort()
	if err != nil {
		return "", fmt.Errorf("generating code: %w", err)
	}

	switch g.config.Language {
	case "go":
		return g.generateGoCode(pipeline, registry, order), nil
	case "python":
		return g.generatePythonCode(pipeline, registry, order), nil
	case "featherql":
		return g.generateFeatherQLCode(pipeline, registry, order), nil
	default:
		return "", fmt.Errorf("unsupported language %q", g.config.Language)
	}
}

func (g *CodeGenerator) generateGoCode(p *Pipeline, reg *TransformRegistry, order []string) string {
	var b strings.Builder
	pkg := g.config.PackageName
	if pkg == "" {
		pkg = "pipeline"
	}

	b.WriteString(fmt.Sprintf("package %s\n\n", pkg))
	if g.config.IncludeComments {
		b.WriteString(fmt.Sprintf("// Pipeline: %s\n", p.Name))
		if p.Description != "" {
			b.WriteString(fmt.Sprintf("// %s\n", p.Description))
		}
		b.WriteString("\n")
	}

	b.WriteString("import (\n\t\"context\"\n\t\"fmt\"\n)\n\n")
	b.WriteString("func RunPipeline(ctx context.Context) error {\n")

	for _, id := range order {
		node := p.Nodes[id]
		varName := sanitizeIdentifier(node.Name)
		if g.config.IncludeComments {
			b.WriteString(fmt.Sprintf("\t// Step: %s (%s)\n", node.Name, node.Type))
		}
		b.WriteString(fmt.Sprintf("\t%s, err := execute%s(ctx", varName, strings.Title(string(node.Type))))
		for _, inp := range node.Inputs {
			if inNode, ok := p.Nodes[inp]; ok {
				b.WriteString(fmt.Sprintf(", %s", sanitizeIdentifier(inNode.Name)))
			}
		}
		b.WriteString(")\n")
		b.WriteString(fmt.Sprintf("\tif err != nil {\n\t\treturn fmt.Errorf(\"%s: %%w\", err)\n\t}\n", node.Name))
		b.WriteString(fmt.Sprintf("\t_ = %s\n\n", varName))
	}

	b.WriteString("\treturn nil\n}\n")
	return b.String()
}

func (g *CodeGenerator) generatePythonCode(p *Pipeline, _ *TransformRegistry, order []string) string {
	var b strings.Builder

	if g.config.IncludeComments {
		b.WriteString(fmt.Sprintf("# Pipeline: %s\n", p.Name))
		if p.Description != "" {
			b.WriteString(fmt.Sprintf("# %s\n", p.Description))
		}
		b.WriteString("\n")
	}

	b.WriteString("def run_pipeline():\n")
	b.WriteString("    results = {}\n\n")

	for _, id := range order {
		node := p.Nodes[id]
		if g.config.IncludeComments {
			b.WriteString(fmt.Sprintf("    # Step: %s (%s)\n", node.Name, node.Type))
		}
		inputs := "[]"
		if len(node.Inputs) > 0 {
			parts := make([]string, len(node.Inputs))
			for i, inp := range node.Inputs {
				parts[i] = fmt.Sprintf("results[\"%s\"]", inp)
			}
			inputs = "[" + strings.Join(parts, ", ") + "]"
		}
		b.WriteString(fmt.Sprintf("    results[\"%s\"] = execute_%s(\"%s\", %s)\n\n", id, node.Type, node.Name, inputs))
	}

	b.WriteString("    return results\n")
	return b.String()
}

func (g *CodeGenerator) generateFeatherQLCode(p *Pipeline, _ *TransformRegistry, order []string) string {
	var b strings.Builder

	if g.config.IncludeComments {
		b.WriteString(fmt.Sprintf("-- Pipeline: %s\n", p.Name))
		if p.Description != "" {
			b.WriteString(fmt.Sprintf("-- %s\n", p.Description))
		}
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf("CREATE PIPELINE %s AS\n", sanitizeIdentifier(p.Name)))

	for i, id := range order {
		node := p.Nodes[id]
		if i > 0 {
			b.WriteString("  | ")
		} else {
			b.WriteString("  ")
		}
		b.WriteString(fmt.Sprintf("%s(\"%s\"", strings.ToUpper(string(node.Type)), node.Name))
		if len(node.Inputs) > 0 {
			b.WriteString(fmt.Sprintf(", FROM %s", strings.Join(node.Inputs, ", ")))
		}
		b.WriteString(")\n")
	}

	b.WriteString(";\n")
	return b.String()
}

func sanitizeIdentifier(s string) string {
	r := strings.NewReplacer(" ", "_", "-", "_", ".", "_")
	return r.Replace(strings.ToLower(s))
}

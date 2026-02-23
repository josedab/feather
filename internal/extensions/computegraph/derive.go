package computegraph

import (
	"fmt"
	"strings"
	"time"
)

// WindowSpec describes a windowing strategy for a derived feature.
type WindowSpec struct {
	Type     string        `json:"type"`     // tumbling, sliding, session
	Duration time.Duration `json:"duration"` // window size
	Slide    time.Duration `json:"slide"`    // slide interval for sliding windows
}

// DeriveSpec is a declarative specification for a derived feature node.
type DeriveSpec struct {
	Name       string            `json:"name"`
	Expression string            `json:"expression"`
	Inputs     []string          `json:"inputs"`
	Function   ComputeFunc       `json:"function"`
	OutputType string            `json:"output_type"`
	Policy     MaterializePolicy `json:"materialize_policy"`
	Window     *WindowSpec       `json:"window,omitempty"`
}

// ParseDeriveStatement parses a DERIVE statement string into a DeriveSpec.
//
// Syntax:
//
//	DERIVE <name> AS <expression> FROM <input1>, <input2> [USING <function>] [POLICY <policy>] [WINDOW <type> <duration> [SLIDE <slide>]]
func ParseDeriveStatement(stmt string) (*DeriveSpec, error) {
	tokens := strings.Fields(strings.TrimSpace(stmt))
	if len(tokens) < 2 || strings.ToUpper(tokens[0]) != "DERIVE" {
		return nil, fmt.Errorf("parsing derive: statement must start with DERIVE")
	}

	spec := &DeriveSpec{
		Name:     tokens[1],
		Function: FuncIdentity,
	}

	asIdx := findTokenCI(tokens, "AS")
	fromIdx := findTokenCI(tokens, "FROM")

	if asIdx < 0 || fromIdx < 0 {
		return nil, fmt.Errorf("parsing derive %q: expected AS and FROM clauses", spec.Name)
	}

	if fromIdx <= asIdx {
		return nil, fmt.Errorf("parsing derive %q: FROM must appear after AS", spec.Name)
	}

	// Expression is between AS and FROM.
	spec.Expression = strings.Join(tokens[asIdx+1:fromIdx], " ")
	if spec.Expression == "" {
		return nil, fmt.Errorf("parsing derive %q: empty expression", spec.Name)
	}

	// Determine end of input list.
	usingIdx := findTokenCI(tokens, "USING")
	policyIdx := findTokenCI(tokens, "POLICY")
	windowIdx := findTokenCI(tokens, "WINDOW")

	inputEnd := len(tokens)
	for _, idx := range []int{usingIdx, policyIdx, windowIdx} {
		if idx > fromIdx && idx < inputEnd {
			inputEnd = idx
		}
	}

	inputStr := strings.Join(tokens[fromIdx+1:inputEnd], " ")
	for _, inp := range strings.Split(inputStr, ",") {
		inp = strings.TrimSpace(inp)
		if inp != "" {
			spec.Inputs = append(spec.Inputs, inp)
		}
	}
	if len(spec.Inputs) == 0 {
		return nil, fmt.Errorf("parsing derive %q: at least one input required", spec.Name)
	}

	// Optional USING clause.
	if usingIdx > 0 && usingIdx+1 < len(tokens) {
		spec.Function = parseComputeFunc(tokens[usingIdx+1])
	}

	// Optional POLICY clause.
	if policyIdx > 0 && policyIdx+1 < len(tokens) {
		spec.Policy = parseMaterializePolicy(tokens[policyIdx+1])
	}

	// Optional WINDOW clause: WINDOW <type> <duration> [SLIDE <duration>]
	if windowIdx > 0 && windowIdx+2 < len(tokens) {
		ws, err := parseWindowClause(tokens[windowIdx+1:])
		if err != nil {
			return nil, fmt.Errorf("parsing derive %q window: %w", spec.Name, err)
		}
		spec.Window = ws
	}

	// Infer output type from expression if not set elsewhere.
	if spec.OutputType == "" {
		spec.OutputType = "float64"
	}

	return spec, nil
}

// parseWindowClause parses tokens after WINDOW keyword.
func parseWindowClause(tokens []string) (*WindowSpec, error) {
	if len(tokens) < 2 {
		return nil, fmt.Errorf("WINDOW requires type and duration")
	}

	wt := strings.ToLower(tokens[0])
	switch wt {
	case "tumbling", "sliding", "session":
	default:
		return nil, fmt.Errorf("unknown window type %q", wt)
	}

	dur, err := time.ParseDuration(tokens[1])
	if err != nil {
		return nil, fmt.Errorf("parsing window duration: %w", err)
	}

	ws := &WindowSpec{
		Type:     wt,
		Duration: dur,
	}

	// Optional SLIDE for sliding windows.
	slideIdx := findTokenCI(tokens, "SLIDE")
	if slideIdx > 0 && slideIdx+1 < len(tokens) {
		slide, err := time.ParseDuration(tokens[slideIdx+1])
		if err != nil {
			return nil, fmt.Errorf("parsing slide duration: %w", err)
		}
		ws.Slide = slide
	}

	return ws, nil
}

// BuildDeriveGraph converts a list of DeriveSpec into FeatureNodes and adds
// them to the engine. Source nodes referenced in DeriveSpec.Inputs must already
// exist in the engine or appear as an earlier spec's output.
func BuildDeriveGraph(engine *Engine, specs []DeriveSpec) (*ApplyResult, error) {
	if engine == nil {
		return nil, fmt.Errorf("building derive graph: nil engine")
	}

	result := &ApplyResult{
		GraphName: "derive",
	}

	for _, spec := range specs {
		if spec.Name == "" {
			result.Errors = append(result.Errors, "derive spec with empty name")
			continue
		}

		node := FeatureNode{
			Name:       spec.Name,
			Kind:       KindDerived,
			Inputs:     spec.Inputs,
			Function:   spec.Function,
			Expression: spec.Expression,
			OutputType: spec.OutputType,
			Policy:     spec.Policy,
			Metadata:   make(map[string]string),
		}

		if spec.Window != nil {
			node.Metadata["window_type"] = spec.Window.Type
			node.Metadata["window_duration"] = spec.Window.Duration.String()
			if spec.Window.Slide > 0 {
				node.Metadata["window_slide"] = spec.Window.Slide.String()
			}
		}

		if err := engine.AddNode(node); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("node %q: %v", spec.Name, err))
			continue
		}
		result.NodesAdded = append(result.NodesAdded, spec.Name)
	}

	result.Success = len(result.Errors) == 0
	return result, nil
}

// DeriveSpecFromDSLNode converts a DSL NodeDefinition into a DeriveSpec,
// bridging the DSL parser with the derive graph builder.
func DeriveSpecFromDSLNode(nd NodeDefinition) DeriveSpec {
	return DeriveSpec{
		Name:       nd.Name,
		Expression: nd.Expression,
		Inputs:     nd.Inputs,
		Function:   parseComputeFunc(nd.Function),
		OutputType: nd.OutputType,
		Policy:     parseMaterializePolicy(nd.Policy),
	}
}

// findTokenCI returns the index of the first case-insensitive match, or -1.
func findTokenCI(tokens []string, keyword string) int {
	upper := strings.ToUpper(keyword)
	for i, t := range tokens {
		if strings.ToUpper(t) == upper {
			return i
		}
	}
	return -1
}

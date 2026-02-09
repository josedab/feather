package computegraph

import (
	"fmt"
	"strings"
)

// GraphDefinition represents a declarative graph definition parsed from YAML/DSL.
type GraphDefinition struct {
	Name        string                 `json:"name" yaml:"name"`
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Nodes       []NodeDefinition       `json:"nodes" yaml:"nodes"`
	Metadata    map[string]string      `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// NodeDefinition is a single node in a declarative graph definition.
type NodeDefinition struct {
	Name        string            `json:"name" yaml:"name"`
	Kind        string            `json:"kind" yaml:"kind"`
	Inputs      []string          `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Function    string            `json:"function" yaml:"function"`
	Expression  string            `json:"expression,omitempty" yaml:"expression,omitempty"`
	OutputType  string            `json:"output_type" yaml:"output_type"`
	Policy      string            `json:"policy,omitempty" yaml:"policy,omitempty"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// ParseDSL parses a simple FeatherQL-like DSL string into a GraphDefinition.
//
// Syntax:
//
//	GRAPH <name>
//	  SOURCE <node> AS <output_type>
//	  DERIVE <node> FROM <input1>, <input2> USING <function> AS <output_type>
//	  DERIVE <node> FROM <input1> USING <function> AS <output_type> [POLICY <lazy|eager|scheduled>]
//	END
func ParseDSL(input string) (*GraphDefinition, error) {
	lines := strings.Split(input, "\n")
	def := &GraphDefinition{}
	inGraph := false

	for i, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "--") || strings.HasPrefix(line, "#") {
			continue
		}

		tokens := tokenizeLine(line)
		if len(tokens) == 0 {
			continue
		}

		upper := strings.ToUpper(tokens[0])

		switch {
		case upper == "GRAPH" && !inGraph:
			if len(tokens) < 2 {
				return nil, fmt.Errorf("line %d: GRAPH requires a name", i+1)
			}
			def.Name = tokens[1]
			inGraph = true

		case upper == "END" && inGraph:
			inGraph = false

		case upper == "SOURCE" && inGraph:
			node, err := parseSourceLine(tokens, i+1)
			if err != nil {
				return nil, err
			}
			def.Nodes = append(def.Nodes, node)

		case upper == "DERIVE" && inGraph:
			node, err := parseDeriveLine(tokens, i+1)
			if err != nil {
				return nil, err
			}
			def.Nodes = append(def.Nodes, node)

		default:
			if inGraph {
				return nil, fmt.Errorf("line %d: unexpected token %q", i+1, tokens[0])
			}
		}
	}

	if inGraph {
		return nil, fmt.Errorf("unterminated GRAPH block (missing END)")
	}

	return def, nil
}

// parseSourceLine: SOURCE <name> AS <output_type>
func parseSourceLine(tokens []string, lineNum int) (NodeDefinition, error) {
	node := NodeDefinition{Kind: "source", Function: "identity"}
	if len(tokens) < 4 || strings.ToUpper(tokens[2]) != "AS" {
		return node, fmt.Errorf("line %d: SOURCE syntax: SOURCE <name> AS <type>", lineNum)
	}
	node.Name = tokens[1]
	node.OutputType = tokens[3]
	return node, nil
}

// parseDeriveLine: DERIVE <name> FROM <inputs> USING <function> AS <type> [POLICY <policy>]
func parseDeriveLine(tokens []string, lineNum int) (NodeDefinition, error) {
	node := NodeDefinition{Kind: "derived"}
	if len(tokens) < 2 {
		return node, fmt.Errorf("line %d: DERIVE requires a name", lineNum)
	}
	node.Name = tokens[1]

	fromIdx := findToken(tokens, "FROM")
	usingIdx := findToken(tokens, "USING")
	asIdx := findToken(tokens, "AS")
	policyIdx := findToken(tokens, "POLICY")

	if fromIdx < 0 || usingIdx < 0 || asIdx < 0 {
		return node, fmt.Errorf("line %d: DERIVE syntax: DERIVE <name> FROM <inputs> USING <func> AS <type>", lineNum)
	}

	// Parse inputs between FROM and USING
	inputStr := strings.Join(tokens[fromIdx+1:usingIdx], " ")
	for _, inp := range strings.Split(inputStr, ",") {
		inp = strings.TrimSpace(inp)
		if inp != "" {
			node.Inputs = append(node.Inputs, inp)
		}
	}

	node.Function = tokens[usingIdx+1]
	if asIdx+1 < len(tokens) {
		node.OutputType = tokens[asIdx+1]
	}

	if policyIdx > 0 && policyIdx+1 < len(tokens) {
		node.Policy = strings.ToLower(tokens[policyIdx+1])
	}

	return node, nil
}

func findToken(tokens []string, keyword string) int {
	for i, t := range tokens {
		if strings.ToUpper(t) == keyword {
			return i
		}
	}
	return -1
}

func tokenizeLine(line string) []string {
	var tokens []string
	for _, part := range strings.Fields(line) {
		tokens = append(tokens, part)
	}
	return tokens
}

// ApplyDefinition applies a GraphDefinition to an Engine, adding all nodes.
func (e *Engine) ApplyDefinition(def *GraphDefinition) (*ApplyResult, error) {
	if def == nil {
		return nil, fmt.Errorf("nil graph definition")
	}

	result := &ApplyResult{
		GraphName: def.Name,
	}

	for _, nd := range def.Nodes {
		kind := parseNodeKind(nd.Kind)
		fn := parseComputeFunc(nd.Function)
		policy := parseMaterializePolicy(nd.Policy)

		node := FeatureNode{
			Name:        nd.Name,
			Kind:        kind,
			Inputs:      nd.Inputs,
			Function:    fn,
			Expression:  nd.Expression,
			OutputType:  nd.OutputType,
			Policy:      policy,
			Description: nd.Description,
			Metadata:    nd.Metadata,
		}

		if err := e.AddNode(node); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("node %q: %v", nd.Name, err))
			continue
		}
		result.NodesAdded = append(result.NodesAdded, nd.Name)
	}

	result.Success = len(result.Errors) == 0
	return result, nil
}

// ApplyResult describes the outcome of applying a graph definition.
type ApplyResult struct {
	GraphName  string   `json:"graph_name"`
	NodesAdded []string `json:"nodes_added"`
	Errors     []string `json:"errors,omitempty"`
	Success    bool     `json:"success"`
}

func parseNodeKind(s string) NodeKind {
	switch strings.ToLower(s) {
	case "source":
		return KindSource
	case "derived":
		return KindDerived
	case "aggregated":
		return KindAggregated
	default:
		return KindDerived
	}
}

func parseComputeFunc(s string) ComputeFunc {
	switch strings.ToLower(s) {
	case "sum":
		return FuncSum
	case "avg":
		return FuncAvg
	case "multiply":
		return FuncMultiply
	case "divide":
		return FuncDivide
	case "concat":
		return FuncConcat
	case "coalesce":
		return FuncCoalesce
	case "identity":
		return FuncIdentity
	case "custom_expr":
		return FuncCustomExpr
	default:
		return FuncIdentity
	}
}

func parseMaterializePolicy(s string) MaterializePolicy {
	switch strings.ToLower(s) {
	case "eager":
		return PolicyEager
	case "lazy":
		return PolicyLazy
	case "scheduled":
		return PolicyScheduled
	default:
		return ""
	}
}

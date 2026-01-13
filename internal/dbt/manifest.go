package dbt

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ParseManifest parses a dbt manifest.json file from the given path.
func ParseManifest(path string) (*Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening manifest file: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	return ParseManifestFromReader(f)
}

// ParseManifestFromReader parses a dbt manifest from an io.Reader.
func ParseManifestFromReader(r io.Reader) (*Manifest, error) {
	var manifest Manifest
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("decoding manifest: %w", err)
	}

	return &manifest, nil
}

// ParseManifestFromBytes parses a dbt manifest from JSON bytes.
func ParseManifestFromBytes(data []byte) (*Manifest, error) {
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("unmarshaling manifest: %w", err)
	}

	return &manifest, nil
}

// GetModels returns all model nodes from the manifest.
func (m *Manifest) GetModels() []Node {
	return m.GetNodesByType("model")
}

// GetSeeds returns all seed nodes from the manifest.
func (m *Manifest) GetSeeds() []Node {
	return m.GetNodesByType("seed")
}

// GetSnapshots returns all snapshot nodes from the manifest.
func (m *Manifest) GetSnapshots() []Node {
	return m.GetNodesByType("snapshot")
}

// GetNodesByType returns all nodes of a specific type.
func (m *Manifest) GetNodesByType(resourceType string) []Node {
	var nodes []Node
	for _, node := range m.Nodes {
		if node.ResourceType == resourceType {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// GetNode returns a node by its unique ID.
func (m *Manifest) GetNode(uniqueID string) (*Node, bool) {
	node, ok := m.Nodes[uniqueID]
	if !ok {
		return nil, false
	}
	return &node, true
}

// GetSource returns a source by its unique ID.
func (m *Manifest) GetSource(uniqueID string) (*Source, bool) {
	source, ok := m.Sources[uniqueID]
	if !ok {
		return nil, false
	}
	return &source, true
}

// GetMetric returns a metric by its unique ID.
func (m *Manifest) GetMetric(uniqueID string) (*Metric, bool) {
	metric, ok := m.Metrics[uniqueID]
	if !ok {
		return nil, false
	}
	return &metric, true
}

// FilterModels filters models by tags and name patterns.
func (m *Manifest) FilterModels(tags []string, patterns []string) []Node {
	models := m.GetModels()
	if len(tags) == 0 && len(patterns) == 0 {
		return models
	}

	var filtered []Node
	for _, model := range models {
		if matchesTags(model.Tags, tags) && matchesPatterns(model.Name, patterns) {
			filtered = append(filtered, model)
		}
	}

	return filtered
}

// matchesTags checks if the node has any of the specified tags.
func matchesTags(nodeTags, filterTags []string) bool {
	if len(filterTags) == 0 {
		return true
	}

	tagSet := make(map[string]bool)
	for _, t := range nodeTags {
		tagSet[t] = true
	}

	for _, t := range filterTags {
		if tagSet[t] {
			return true
		}
	}

	return false
}

// matchesPatterns checks if the name matches any of the patterns.
func matchesPatterns(name string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}

	for _, pattern := range patterns {
		matched, _ := filepath.Match(pattern, name)
		if matched {
			return true
		}
		// Also check for simple substring match
		if strings.Contains(name, pattern) {
			return true
		}
	}

	return false
}

// GetUpstreamNodes returns all upstream dependencies for a node.
func (m *Manifest) GetUpstreamNodes(uniqueID string) []Node {
	node, ok := m.GetNode(uniqueID)
	if !ok {
		return nil
	}

	var upstream []Node
	for _, depID := range node.DependsOn.Nodes {
		if depNode, found := m.GetNode(depID); found {
			upstream = append(upstream, *depNode)
		}
	}

	return upstream
}

// GetDownstreamNodes returns all nodes that depend on the given node.
func (m *Manifest) GetDownstreamNodes(uniqueID string) []Node {
	var downstream []Node
	for _, node := range m.Nodes {
		for _, depID := range node.DependsOn.Nodes {
			if depID == uniqueID {
				downstream = append(downstream, node)
				break
			}
		}
	}

	return downstream
}

// Lineage contains upstream and downstream dependencies for a node.
type Lineage struct {
	Node       Node   `json:"node"`
	Upstream   []Node `json:"upstream"`
	Downstream []Node `json:"downstream"`
}

// GetLineage returns the lineage for a given node.
func (m *Manifest) GetLineage(uniqueID string) (*Lineage, error) {
	node, ok := m.GetNode(uniqueID)
	if !ok {
		return nil, fmt.Errorf("node not found: %s", uniqueID)
	}

	return &Lineage{
		Node:       *node,
		Upstream:   m.GetUpstreamNodes(uniqueID),
		Downstream: m.GetDownstreamNodes(uniqueID),
	}, nil
}

// Validate performs basic validation on the manifest.
func (m *Manifest) Validate() error {
	if m.Metadata.DBTSchemaVersion == "" {
		return fmt.Errorf("missing dbt_schema_version in manifest metadata")
	}

	// Check for circular dependencies (basic check)
	visited := make(map[string]bool)
	var checkCycle func(nodeID string, path []string) error

	checkCycle = func(nodeID string, path []string) error {
		for _, p := range path {
			if p == nodeID {
				return fmt.Errorf("circular dependency detected: %s", strings.Join(append(path, nodeID), " -> "))
			}
		}

		if visited[nodeID] {
			return nil
		}
		visited[nodeID] = true

		node, ok := m.GetNode(nodeID)
		if !ok {
			return nil
		}

		newPath := append(path, nodeID)
		for _, depID := range node.DependsOn.Nodes {
			if err := checkCycle(depID, newPath); err != nil {
				return err
			}
		}

		return nil
	}

	for nodeID := range m.Nodes {
		if err := checkCycle(nodeID, nil); err != nil {
			return err
		}
	}

	return nil
}

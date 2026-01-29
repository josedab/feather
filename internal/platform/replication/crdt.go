package replication

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"
)

// CRDTType identifies the type of conflict-free replicated data type.
type CRDTType string

const (
	CRDTLWWRegister CRDTType = "lww_register"
	CRDTGCounter    CRDTType = "g_counter"
	CRDTPNCounter   CRDTType = "pn_counter"
	CRDTORSet       CRDTType = "or_set"
)

// LWWRegister implements a Last-Writer-Wins Register CRDT.
type LWWRegister struct {
	Value     interface{} `json:"value"`
	Timestamp int64       `json:"timestamp"`
	RegionID  string      `json:"region_id"`
}

// Merge merges two LWW registers, keeping the latest write.
func (r *LWWRegister) Merge(other *LWWRegister) *LWWRegister {
	if other.Timestamp > r.Timestamp {
		return other
	}
	if other.Timestamp == r.Timestamp && other.RegionID > r.RegionID {
		return other
	}
	return r
}

// GCounter implements a Grow-only Counter CRDT.
type GCounter struct {
	Counts map[string]uint64 `json:"counts"`
}

// NewGCounter creates a new G-Counter.
func NewGCounter() *GCounter {
	return &GCounter{Counts: make(map[string]uint64)}
}

// Increment adds to the counter for a specific region.
func (c *GCounter) Increment(regionID string, amount uint64) {
	c.Counts[regionID] += amount
}

// Value returns the total count across all regions.
func (c *GCounter) Value() uint64 {
	var total uint64
	for _, v := range c.Counts {
		total += v
	}
	return total
}

// Merge merges two G-Counters by taking the max for each region.
func (c *GCounter) Merge(other *GCounter) *GCounter {
	result := NewGCounter()
	for k, v := range c.Counts {
		result.Counts[k] = v
	}
	for k, v := range other.Counts {
		if v > result.Counts[k] {
			result.Counts[k] = v
		}
	}
	return result
}

// MerkleNode represents a node in a Merkle tree for anti-entropy.
type MerkleNode struct {
	Hash     string        `json:"hash"`
	Level    int           `json:"level"`
	KeyRange [2]string     `json:"key_range"`
	Children []*MerkleNode `json:"children,omitempty"`
}

// MerkleTree provides efficient consistency verification between regions.
type MerkleTree struct {
	mu   sync.RWMutex
	root *MerkleNode
	data map[string]string // key -> hash of value
}

// NewMerkleTree creates a new Merkle tree.
func NewMerkleTree() *MerkleTree {
	return &MerkleTree{
		data: make(map[string]string),
		root: &MerkleNode{Level: 0},
	}
}

// Insert adds or updates a key-value hash in the tree.
func (t *MerkleTree) Insert(key string, valueHash string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.data[key] = valueHash
	t.rebuildRoot()
}

// RootHash returns the hash of the entire tree.
func (t *MerkleTree) RootHash() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.root.Hash
}

// Diff compares two Merkle trees and returns keys that differ.
func (t *MerkleTree) Diff(otherData map[string]string) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var diffs []string
	seen := make(map[string]bool)

	for k, v := range t.data {
		seen[k] = true
		if ov, ok := otherData[k]; !ok || ov != v {
			diffs = append(diffs, k)
		}
	}
	for k := range otherData {
		if !seen[k] {
			diffs = append(diffs, k)
		}
	}

	sort.Strings(diffs)
	return diffs
}

// DataSnapshot returns a copy of all key-hash pairs.
func (t *MerkleTree) DataSnapshot() map[string]string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	snapshot := make(map[string]string, len(t.data))
	for k, v := range t.data {
		snapshot[k] = v
	}
	return snapshot
}

func (t *MerkleTree) rebuildRoot() {
	h := sha256.New()
	keys := make([]string, 0, len(t.data))
	for k := range t.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		h.Write([]byte(k + ":" + t.data[k]))
	}
	t.root.Hash = hex.EncodeToString(h.Sum(nil))
}

// ConflictResolution holds the result of a conflict resolution.
type ConflictResolution struct {
	Key          string      `json:"key"`
	WinnerRegion string      `json:"winner_region"`
	WinnerValue  interface{} `json:"winner_value"`
	LoserRegions []string    `json:"loser_regions,omitempty"`
	Strategy     string      `json:"strategy"`
	ResolvedAt   time.Time   `json:"resolved_at"`
}

// ConflictResolver handles multi-region write conflicts.
type ConflictResolver struct {
	mu         sync.RWMutex
	policy     ConflictPolicy
	history    []ConflictResolution
	historyMax int
}

// NewConflictResolver creates a resolver with the given policy.
func NewConflictResolver(policy ConflictPolicy) *ConflictResolver {
	return &ConflictResolver{
		policy:     policy,
		historyMax: 1000,
	}
}

// Resolve resolves a conflict between multiple replicated values.
func (r *ConflictResolver) Resolve(key string, values []*ReplicatedValue) (*ConflictResolution, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("resolving conflict: no values provided")
	}
	if len(values) == 1 {
		return &ConflictResolution{
			Key:          key,
			WinnerRegion: values[0].Origin,
			WinnerValue:  values[0].Value,
			Strategy:     "single_value",
			ResolvedAt:   time.Now(),
		}, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	var resolution *ConflictResolution

	switch r.policy {
	case PolicyLastWriterWins:
		resolution = r.resolveLWW(key, values)
	case PolicyHighestVersion:
		resolution = r.resolveHighestVersion(key, values)
	default:
		resolution = r.resolveLWW(key, values)
	}

	r.history = append(r.history, *resolution)
	if len(r.history) > r.historyMax {
		r.history = r.history[len(r.history)-r.historyMax:]
	}

	return resolution, nil
}

// History returns recent conflict resolutions.
func (r *ConflictResolver) History() []ConflictResolution {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ConflictResolution, len(r.history))
	copy(result, r.history)
	return result
}

func (r *ConflictResolver) resolveLWW(key string, values []*ReplicatedValue) *ConflictResolution {
	winner := values[0]
	for _, v := range values[1:] {
		if clockSum(v.Clock) > clockSum(winner.Clock) {
			winner = v
		}
	}

	var losers []string
	for _, v := range values {
		if v.Origin != winner.Origin {
			losers = append(losers, v.Origin)
		}
	}

	return &ConflictResolution{
		Key:          key,
		WinnerRegion: winner.Origin,
		WinnerValue:  winner.Value,
		LoserRegions: losers,
		Strategy:     string(PolicyLastWriterWins),
		ResolvedAt:   time.Now(),
	}
}

func (r *ConflictResolver) resolveHighestVersion(key string, values []*ReplicatedValue) *ConflictResolution {
	winner := values[0]
	winnerSum := clockSum(winner.Clock)
	for _, v := range values[1:] {
		vSum := clockSum(v.Clock)
		if vSum > winnerSum {
			winner = v
			winnerSum = vSum
		}
	}

	var losers []string
	for _, v := range values {
		if v.Origin != winner.Origin {
			losers = append(losers, v.Origin)
		}
	}

	return &ConflictResolution{
		Key:          key,
		WinnerRegion: winner.Origin,
		WinnerValue:  winner.Value,
		LoserRegions: losers,
		Strategy:     string(PolicyHighestVersion),
		ResolvedAt:   time.Now(),
	}
}

// DataResidencyPolicy defines geo-compliance rules.
type DataResidencyPolicy struct {
	Name           string   `json:"name"`
	AllowedRegions []string `json:"allowed_regions"`
	DeniedRegions  []string `json:"denied_regions,omitempty"`
	EntityPattern  string   `json:"entity_pattern,omitempty"`
}

// DataResidencyChecker validates data residency compliance.
type DataResidencyChecker struct {
	mu       sync.RWMutex
	policies []DataResidencyPolicy
}

// NewDataResidencyChecker creates a new residency checker.
func NewDataResidencyChecker() *DataResidencyChecker {
	return &DataResidencyChecker{}
}

// AddPolicy registers a data residency policy.
func (c *DataResidencyChecker) AddPolicy(policy DataResidencyPolicy) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.policies = append(c.policies, policy)
}

// CheckCompliance verifies if storing data in a region is compliant.
func (c *DataResidencyChecker) CheckCompliance(regionID string) (bool, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, p := range c.policies {
		for _, denied := range p.DeniedRegions {
			if denied == regionID {
				return false, fmt.Sprintf("region %q denied by policy %q", regionID, p.Name)
			}
		}
		if len(p.AllowedRegions) > 0 {
			allowed := false
			for _, a := range p.AllowedRegions {
				if a == regionID {
					allowed = true
					break
				}
			}
			if !allowed {
				return false, fmt.Sprintf("region %q not in allowed list for policy %q", regionID, p.Name)
			}
		}
	}

	return true, ""
}

// ListPolicies returns all registered policies.
func (c *DataResidencyChecker) ListPolicies() []DataResidencyPolicy {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]DataResidencyPolicy, len(c.policies))
	copy(result, c.policies)
	return result
}

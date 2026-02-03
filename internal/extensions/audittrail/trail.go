// Package audittrail provides an event-sourced audit trail with Merkle tree
// cryptographic hash chaining for immutable, append-only event logging.
package audittrail

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ActionType represents the kind of auditable action.
type ActionType string

const (
	ActionCreate       ActionType = "create"
	ActionRead         ActionType = "read"
	ActionUpdate       ActionType = "update"
	ActionDelete       ActionType = "delete"
	ActionSchemaChange ActionType = "schema_change"
)

// Config holds configuration for the audit trail.
type Config struct {
	MaxEvents                int
	HashAlgorithm            string
	EnableCryptographicProof bool
	ComplianceMode           string // "sox", "hipaa", "gdpr"
	RetentionDays            int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxEvents:                1_000_000,
		HashAlgorithm:            "sha256",
		EnableCryptographicProof: true,
		ComplianceMode:           "sox",
		RetentionDays:            365,
	}
}

// Event represents a single auditable action in the system.
type Event struct {
	ID           string      `json:"id"`
	Action       ActionType  `json:"action"`
	Entity       string      `json:"entity"`
	Feature      string      `json:"feature"`
	Actor        string      `json:"actor"`
	Timestamp    time.Time   `json:"timestamp"`
	Value        interface{} `json:"value,omitempty"`
	PreviousHash string      `json:"previous_hash"`
	Hash         string      `json:"hash"`
}

// IntegrityProof is a cryptographic attestation for a single event.
type IntegrityProof struct {
	EventID      string `json:"event_id"`
	Hash         string `json:"hash"`
	PreviousHash string `json:"previous_hash"`
	MerkleRoot   string `json:"merkle_root"`
	ChainLength  int    `json:"chain_length"`
	Verified     bool   `json:"verified"`
}

// ComplianceReport summarises audit activity over a period.
type ComplianceReport struct {
	Type            string    `json:"type"`
	Period          string    `json:"period"`
	TotalEvents     int       `json:"total_events"`
	AccessLogs      []Event   `json:"access_logs"`
	MutationHistory []Event   `json:"mutation_history"`
	IntegrityStatus string    `json:"integrity_status"`
	GeneratedAt     time.Time `json:"generated_at"`
}

// TrailStats exposes high-level statistics about the trail.
type TrailStats struct {
	TotalEvents       int       `json:"total_events"`
	ChainLength       int       `json:"chain_length"`
	MerkleRoot        string    `json:"merkle_root"`
	LastEventTime     time.Time `json:"last_event_time"`
	IntegrityVerified bool      `json:"integrity_verified"`
}

// Trail is an append-only, hash-chained audit log. All methods are safe for
// concurrent use.
type Trail struct {
	mu     sync.RWMutex
	config Config
	events []Event

	// indexes for fast lookups
	byEntity  map[string][]int
	byFeature map[string][]int
	byActor   map[string][]int
	byID      map[string]int
}

// New creates a Trail with the supplied configuration.
func New(cfg Config) *Trail {
	return &Trail{
		config:    cfg,
		events:    make([]Event, 0, 1024),
		byEntity:  make(map[string][]int),
		byFeature: make(map[string][]int),
		byActor:   make(map[string][]int),
		byID:      make(map[string]int),
	}
}

// ComputeHash returns the SHA-256 digest of the event data chained to the
// previous hash. The hash covers all semantically relevant fields; the Hash
// field itself is excluded to avoid circular dependency.
func ComputeHash(e *Event) string {
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s",
		e.PreviousHash,
		e.ID,
		e.Action,
		e.Entity,
		e.Feature,
		e.Actor,
		e.Timestamp.UTC().Format(time.RFC3339Nano),
	)

	if e.Value != nil {
		if b, err := json.Marshal(e.Value); err == nil {
			data += "|" + string(b)
		}
	}

	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

// Record appends an event to the trail, computing its hash from the chain.
func (t *Trail) Record(e Event) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.events) >= t.config.MaxEvents {
		return fmt.Errorf("audittrail: max events (%d) reached", t.config.MaxEvents)
	}

	if _, exists := t.byID[e.ID]; exists {
		return fmt.Errorf("audittrail: duplicate event ID %q", e.ID)
	}

	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}

	if len(t.events) > 0 {
		e.PreviousHash = t.events[len(t.events)-1].Hash
	}

	e.Hash = ComputeHash(&e)

	idx := len(t.events)
	t.events = append(t.events, e)

	t.byID[e.ID] = idx
	t.byEntity[e.Entity] = append(t.byEntity[e.Entity], idx)
	t.byFeature[e.Feature] = append(t.byFeature[e.Feature], idx)
	t.byActor[e.Actor] = append(t.byActor[e.Actor], idx)

	return nil
}

// RecordMutation is a convenience helper that records an update event with
// old/new values.
func (t *Trail) RecordMutation(entity, feature, actor string, oldValue, newValue interface{}) error {
	e := Event{
		ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		Action:    ActionUpdate,
		Entity:    entity,
		Feature:   feature,
		Actor:     actor,
		Timestamp: time.Now().UTC(),
		Value: map[string]interface{}{
			"old": oldValue,
			"new": newValue,
		},
	}
	return t.Record(e)
}

// VerifyChain walks the entire event log and checks that every hash is
// consistent with its predecessor. It returns false together with a
// descriptive error on the first inconsistency found.
func (t *Trail) VerifyChain() (bool, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for i, e := range t.events {
		if i == 0 {
			if e.PreviousHash != "" {
				return false, fmt.Errorf("audittrail: first event has non-empty previous hash")
			}
		} else if e.PreviousHash != t.events[i-1].Hash {
			return false, fmt.Errorf("audittrail: chain broken at index %d (event %s)", i, e.ID)
		}

		expected := ComputeHash(&e)
		if e.Hash != expected {
			return false, fmt.Errorf("audittrail: hash mismatch at index %d (event %s)", i, e.ID)
		}
	}

	return true, nil
}

// GetMerkleRoot computes a Merkle tree root over the hashes of all recorded
// events and returns it as a hex string. An empty trail returns an empty
// string.
func (t *Trail) GetMerkleRoot() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if len(t.events) == 0 {
		return ""
	}

	hashes := make([][]byte, len(t.events))
	for i, e := range t.events {
		b, _ := hex.DecodeString(e.Hash)
		hashes[i] = b
	}

	return hex.EncodeToString(merkleRoot(hashes))
}

// merkleRoot computes the root of a binary Merkle tree from a slice of leaf
// hashes. The input slice is consumed.
func merkleRoot(hashes [][]byte) []byte {
	if len(hashes) == 1 {
		return hashes[0]
	}

	var next [][]byte
	for i := 0; i < len(hashes); i += 2 {
		if i+1 < len(hashes) {
			h := sha256.Sum256(append(hashes[i], hashes[i+1]...))
			next = append(next, h[:])
		} else {
			// odd leaf is promoted unchanged
			h := sha256.Sum256(append(hashes[i], hashes[i]...))
			next = append(next, h[:])
		}
	}

	return merkleRoot(next)
}

// QueryByEntity returns events for a given entity within the time range.
func (t *Trail) QueryByEntity(entity string, start, end time.Time) []Event {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.filterByIndex(t.byEntity[entity], start, end)
}

// QueryByFeature returns events for a given feature within the time range.
func (t *Trail) QueryByFeature(feature string, start, end time.Time) []Event {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.filterByIndex(t.byFeature[feature], start, end)
}

// QueryByActor returns events for a given actor within the time range.
func (t *Trail) QueryByActor(actor string, start, end time.Time) []Event {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.filterByIndex(t.byActor[actor], start, end)
}

// QueryByTimeRange returns all events within the time range [start, end].
func (t *Trail) QueryByTimeRange(start, end time.Time) []Event {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []Event
	for _, e := range t.events {
		if !e.Timestamp.Before(start) && !e.Timestamp.After(end) {
			result = append(result, e)
		}
	}
	return result
}

func (t *Trail) filterByIndex(indices []int, start, end time.Time) []Event {
	var result []Event
	for _, idx := range indices {
		e := t.events[idx]
		if !e.Timestamp.Before(start) && !e.Timestamp.After(end) {
			result = append(result, e)
		}
	}
	return result
}

// GetProof builds an IntegrityProof for the event with the given ID.
func (t *Trail) GetProof(eventID string) (*IntegrityProof, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	idx, ok := t.byID[eventID]
	if !ok {
		return nil, fmt.Errorf("audittrail: event %q not found", eventID)
	}

	e := t.events[idx]
	expected := ComputeHash(&e)

	proof := &IntegrityProof{
		EventID:      e.ID,
		Hash:         e.Hash,
		PreviousHash: e.PreviousHash,
		MerkleRoot:   t.merkleRootLocked(),
		ChainLength:  len(t.events),
		Verified:     e.Hash == expected,
	}

	return proof, nil
}

// VerifyProof checks that the proof's hash matches its recomputed value and
// that the Merkle root is consistent with the current trail.
func (t *Trail) VerifyProof(proof *IntegrityProof) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	idx, ok := t.byID[proof.EventID]
	if !ok {
		return false
	}

	e := t.events[idx]
	if e.Hash != proof.Hash {
		return false
	}

	if ComputeHash(&e) != proof.Hash {
		return false
	}

	return proof.MerkleRoot == t.merkleRootLocked()
}

// merkleRootLocked computes the Merkle root while the caller already holds at
// least a read lock.
func (t *Trail) merkleRootLocked() string {
	if len(t.events) == 0 {
		return ""
	}

	hashes := make([][]byte, len(t.events))
	for i, e := range t.events {
		b, _ := hex.DecodeString(e.Hash)
		hashes[i] = b
	}

	return hex.EncodeToString(merkleRoot(hashes))
}

// GenerateComplianceReport produces a compliance report for the given type
// ("sox", "hipaa", or "gdpr") over [start, end].
func (t *Trail) GenerateComplianceReport(reportType string, start, end time.Time) (*ComplianceReport, error) {
	switch reportType {
	case "sox", "hipaa", "gdpr":
	default:
		return nil, fmt.Errorf("audittrail: unsupported report type %q", reportType)
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	var accessLogs, mutations []Event
	for _, e := range t.events {
		if e.Timestamp.Before(start) || e.Timestamp.After(end) {
			continue
		}
		if e.Action == ActionRead {
			accessLogs = append(accessLogs, e)
		} else {
			mutations = append(mutations, e)
		}
	}

	integrityStatus := "verified"
	for i, e := range t.events {
		if i == 0 {
			if e.PreviousHash != "" {
				integrityStatus = "compromised"
				break
			}
		} else if e.PreviousHash != t.events[i-1].Hash {
			integrityStatus = "compromised"
			break
		}
		if ComputeHash(&e) != e.Hash {
			integrityStatus = "compromised"
			break
		}
	}

	report := &ComplianceReport{
		Type:            reportType,
		Period:          fmt.Sprintf("%s to %s", start.UTC().Format(time.RFC3339), end.UTC().Format(time.RFC3339)),
		TotalEvents:     len(accessLogs) + len(mutations),
		AccessLogs:      accessLogs,
		MutationHistory: mutations,
		IntegrityStatus: integrityStatus,
		GeneratedAt:     time.Now().UTC(),
	}

	return report, nil
}

// Stats returns high-level statistics about the trail.
func (t *Trail) Stats() TrailStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats := TrailStats{
		TotalEvents: len(t.events),
		ChainLength: len(t.events),
		MerkleRoot:  t.merkleRootLocked(),
	}

	if len(t.events) > 0 {
		stats.LastEventTime = t.events[len(t.events)-1].Timestamp
	}

	// Inline chain verification so we don't double-lock.
	stats.IntegrityVerified = true
	for i, e := range t.events {
		if i == 0 {
			if e.PreviousHash != "" {
				stats.IntegrityVerified = false
				break
			}
		} else if e.PreviousHash != t.events[i-1].Hash {
			stats.IntegrityVerified = false
			break
		}
		if ComputeHash(&e) != e.Hash {
			stats.IntegrityVerified = false
			break
		}
	}

	return stats
}

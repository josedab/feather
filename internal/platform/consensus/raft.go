package consensus

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// RaftState represents the current state of a Raft node.
type RaftState string

const (
	// StateFollower indicates the node is a follower.
	StateFollower RaftState = "follower"
	// StateCandidate indicates the node is a candidate for leader election.
	StateCandidate RaftState = "candidate"
	// StateLeader indicates the node is the cluster leader.
	StateLeader RaftState = "leader"
)

// EntryType represents the type of a log entry.
type EntryType string

const (
	// EntryTypeCommand is a state machine command.
	EntryTypeCommand EntryType = "command"
	// EntryTypeConfig is a configuration change.
	EntryTypeConfig EntryType = "config"
	// EntryTypeNoop is a no-op entry used for leader confirmation.
	EntryTypeNoop EntryType = "noop"
)

// RaftConfig configures a Raft consensus node.
type RaftConfig struct {
	NodeID            string        `json:"node_id"`
	ElectionTimeout   time.Duration `json:"election_timeout"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
	MaxLogEntries     int           `json:"max_log_entries"`
	SnapshotInterval  int           `json:"snapshot_interval"`
	Peers             []string      `json:"peers"`
}

// DefaultRaftConfig returns sensible defaults for a Raft node.
func DefaultRaftConfig() RaftConfig {
	return RaftConfig{
		NodeID:            "",
		ElectionTimeout:   300 * time.Millisecond,
		HeartbeatInterval: 100 * time.Millisecond,
		MaxLogEntries:     10000,
		SnapshotInterval:  1000,
		Peers:             []string{},
	}
}

// LogEntry represents a single entry in the Raft log.
type LogEntry struct {
	Index   uint64    `json:"index"`
	Term    uint64    `json:"term"`
	Type    EntryType `json:"type"`
	Command []byte    `json:"command"`
	Applied time.Time `json:"applied,omitempty"`
}

// StateMachine is the interface that consumers implement to apply
// replicated log entries. It supports snapshotting and restoration.
type StateMachine interface {
	Apply(entry *LogEntry) error
	Snapshot() ([]byte, error)
	Restore(data []byte) error
}

// PeerState tracks the replication state for a single peer.
type PeerState struct {
	ID         string    `json:"id"`
	NextIndex  uint64    `json:"next_index"`
	MatchIndex uint64    `json:"match_index"`
	LastContact time.Time `json:"last_contact"`
	VoteGranted bool     `json:"vote_granted"`
}

// RaftMetrics holds operational metrics for the Raft node.
type RaftMetrics struct {
	TermChanges      uint64 `json:"term_changes"`
	ElectionsStarted uint64 `json:"elections_started"`
	ElectionsWon     uint64 `json:"elections_won"`
	EntriesApplied   uint64 `json:"entries_applied"`
	SnapshotsTaken   uint64 `json:"snapshots_taken"`
}

// RaftNode implements a simplified Raft consensus protocol for
// metadata replication across cluster nodes.
type RaftNode struct {
	config      RaftConfig
	state       RaftState
	currentTerm uint64
	votedFor    string
	log         []*LogEntry
	commitIndex uint64
	lastApplied uint64
	leader      string
	peers       map[string]*PeerState
	stateMachine StateMachine
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	metrics     *RaftMetrics
	started     bool
}

// NewRaftNode creates a new Raft consensus node with the given configuration
// and state machine.
func NewRaftNode(config RaftConfig, sm StateMachine) *RaftNode {
	ctx, cancel := context.WithCancel(context.Background())

	peers := make(map[string]*PeerState)
	for _, peerID := range config.Peers {
		peers[peerID] = &PeerState{
			ID:         peerID,
			NextIndex:  1,
			MatchIndex: 0,
		}
	}

	return &RaftNode{
		config:       config,
		state:        StateFollower,
		currentTerm:  0,
		votedFor:     "",
		log:          make([]*LogEntry, 0),
		commitIndex:  0,
		lastApplied:  0,
		leader:       "",
		peers:        peers,
		stateMachine: sm,
		ctx:          ctx,
		cancel:       cancel,
		metrics:      &RaftMetrics{},
	}
}

// Start begins the Raft protocol loops (election timer and heartbeat).
func (rn *RaftNode) Start(ctx context.Context) error {
	rn.mu.Lock()
	if rn.started {
		rn.mu.Unlock()
		return fmt.Errorf("starting raft node: already started")
	}
	rn.started = true
	rn.mu.Unlock()

	rn.wg.Add(1)
	go rn.electionLoop()

	return nil
}

// Stop gracefully shuts down the Raft node.
func (rn *RaftNode) Stop(ctx context.Context) error {
	rn.mu.Lock()
	if !rn.started {
		rn.mu.Unlock()
		return nil
	}
	rn.started = false
	rn.mu.Unlock()

	rn.cancel()
	rn.wg.Wait()
	return nil
}

// Propose submits a new command to be replicated through the Raft log.
// Only the leader can accept proposals; followers return an error.
func (rn *RaftNode) Propose(ctx context.Context, command []byte) (*LogEntry, error) {
	rn.mu.Lock()
	defer rn.mu.Unlock()

	if rn.state != StateLeader {
		return nil, fmt.Errorf("proposing command: node is not leader (current state: %s)", rn.state)
	}

	entry := &LogEntry{
		Index:   rn.lastLogIndex() + 1,
		Term:    rn.currentTerm,
		Type:    EntryTypeCommand,
		Command: command,
	}

	rn.log = append(rn.log, entry)

	// Trim log if it exceeds maximum size
	if rn.config.MaxLogEntries > 0 && len(rn.log) > rn.config.MaxLogEntries {
		rn.log = rn.log[len(rn.log)-rn.config.MaxLogEntries:]
	}

	// In a simplified model, commit immediately as leader
	rn.commitIndex = entry.Index
	if err := rn.applyEntries(); err != nil {
		return nil, fmt.Errorf("applying proposed entry: %w", err)
	}

	// Update peer next index for replication tracking
	for _, peer := range rn.peers {
		peer.NextIndex = entry.Index + 1
	}

	return entry, nil
}

// GetState returns the current Raft state, term, and whether the node is leader.
func (rn *RaftNode) GetState(ctx context.Context) (RaftState, uint64, bool) {
	rn.mu.RLock()
	defer rn.mu.RUnlock()
	return rn.state, rn.currentTerm, rn.state == StateLeader
}

// GetLeader returns the ID of the current leader.
func (rn *RaftNode) GetLeader(ctx context.Context) string {
	rn.mu.RLock()
	defer rn.mu.RUnlock()
	return rn.leader
}

// IsLeader returns true if this node is the current leader.
func (rn *RaftNode) IsLeader(ctx context.Context) bool {
	rn.mu.RLock()
	defer rn.mu.RUnlock()
	return rn.state == StateLeader
}

// GetLog returns the current Raft log entries.
func (rn *RaftNode) GetLog(ctx context.Context) []*LogEntry {
	rn.mu.RLock()
	defer rn.mu.RUnlock()

	entries := make([]*LogEntry, len(rn.log))
	copy(entries, rn.log)
	return entries
}

// RaftNodeStats holds statistics about the Raft node.
type RaftNodeStats struct {
	NodeID      string     `json:"node_id"`
	State       RaftState  `json:"state"`
	CurrentTerm uint64     `json:"current_term"`
	CommitIndex uint64     `json:"commit_index"`
	LastApplied uint64     `json:"last_applied"`
	LogLength   int        `json:"log_length"`
	Leader      string     `json:"leader"`
	PeerCount   int        `json:"peer_count"`
	Metrics     RaftMetrics `json:"metrics"`
}

// Stats returns operational statistics for the Raft node.
func (rn *RaftNode) Stats(ctx context.Context) RaftNodeStats {
	rn.mu.RLock()
	defer rn.mu.RUnlock()

	return RaftNodeStats{
		NodeID:      rn.config.NodeID,
		State:       rn.state,
		CurrentTerm: rn.currentTerm,
		CommitIndex: rn.commitIndex,
		LastApplied: rn.lastApplied,
		LogLength:   len(rn.log),
		Leader:      rn.leader,
		PeerCount:   len(rn.peers),
		Metrics:     *rn.metrics,
	}
}

// electionLoop runs the election timer. If the timeout fires without
// receiving a heartbeat from a leader, the node starts an election.
func (rn *RaftNode) electionLoop() {
	defer rn.wg.Done()

	timer := time.NewTimer(rn.randomElectionTimeout())
	defer timer.Stop()

	for {
		select {
		case <-rn.ctx.Done():
			return
		case <-timer.C:
			rn.mu.Lock()
			if rn.state != StateLeader {
				rn.startElection()
			}
			rn.mu.Unlock()
			timer.Reset(rn.randomElectionTimeout())
		}
	}
}

// startElection transitions the node to candidate and begins an election.
// Must be called with rn.mu held.
func (rn *RaftNode) startElection() {
	rn.state = StateCandidate
	rn.currentTerm++
	rn.votedFor = rn.config.NodeID
	rn.metrics.TermChanges++
	rn.metrics.ElectionsStarted++

	// Count self vote
	votesReceived := 1
	totalPeers := len(rn.peers)
	majority := (totalPeers+1)/2 + 1

	// In a simplified model, simulate vote collection from peers
	for _, peer := range rn.peers {
		peer.VoteGranted = false
	}

	// If we are the only node (no peers), we win immediately
	if totalPeers == 0 || votesReceived >= majority {
		rn.becomeLeader()
		return
	}

	// Mark vote requests sent; in a real implementation this would
	// send RPCs. Here we remain candidate until the next timeout.
}

// becomeLeader transitions this node to leader state.
// Must be called with rn.mu held.
func (rn *RaftNode) becomeLeader() {
	rn.state = StateLeader
	rn.leader = rn.config.NodeID
	rn.metrics.ElectionsWon++

	// Initialize peer next/match indices
	lastIndex := rn.lastLogIndex()
	for _, peer := range rn.peers {
		peer.NextIndex = lastIndex + 1
		peer.MatchIndex = 0
		peer.LastContact = time.Now()
	}

	// Append a no-op entry to commit entries from previous terms
	noop := &LogEntry{
		Index: lastIndex + 1,
		Term:  rn.currentTerm,
		Type:  EntryTypeNoop,
	}
	rn.log = append(rn.log, noop)
	rn.commitIndex = noop.Index
	_ = rn.applyEntries()
}

// applyEntries applies committed but unapplied log entries to the
// state machine. Must be called with rn.mu held.
func (rn *RaftNode) applyEntries() error {
	for rn.lastApplied < rn.commitIndex {
		rn.lastApplied++
		entry := rn.getEntry(rn.lastApplied)
		if entry == nil {
			continue
		}

		if rn.stateMachine != nil && entry.Type == EntryTypeCommand {
			if err := rn.stateMachine.Apply(entry); err != nil {
				return fmt.Errorf("applying entry %d: %w", entry.Index, err)
			}
		}
		entry.Applied = time.Now()
		rn.metrics.EntriesApplied++
	}
	return nil
}

// getEntry returns the log entry at the given index, or nil.
// Must be called with rn.mu held.
func (rn *RaftNode) getEntry(index uint64) *LogEntry {
	for _, entry := range rn.log {
		if entry.Index == index {
			return entry
		}
	}
	return nil
}

// lastLogIndex returns the index of the last log entry, or 0 if empty.
// Must be called with rn.mu held.
func (rn *RaftNode) lastLogIndex() uint64 {
	if len(rn.log) == 0 {
		return 0
	}
	return rn.log[len(rn.log)-1].Index
}

// randomElectionTimeout returns a randomized election timeout to avoid
// split votes.
func (rn *RaftNode) randomElectionTimeout() time.Duration {
	base := rn.config.ElectionTimeout
	jitter := time.Duration(rand.Int63n(int64(base))) //nolint:gosec
	return base + jitter
}

// GrantVote processes a vote request from a candidate. It grants the vote
// if the candidate's term is at least as current and the node has not voted
// for another candidate in this term. Returns the updated term and whether
// the vote was granted.
func (rn *RaftNode) GrantVote(ctx context.Context, candidateID string, candidateTerm uint64) (uint64, bool) {
	rn.mu.Lock()
	defer rn.mu.Unlock()

	if candidateTerm < rn.currentTerm {
		return rn.currentTerm, false
	}

	if candidateTerm > rn.currentTerm {
		rn.currentTerm = candidateTerm
		rn.state = StateFollower
		rn.votedFor = ""
		rn.leader = ""
		rn.metrics.TermChanges++
	}

	if rn.votedFor == "" || rn.votedFor == candidateID {
		rn.votedFor = candidateID
		return rn.currentTerm, true
	}

	return rn.currentTerm, false
}

// ReceiveHeartbeat processes a heartbeat (AppendEntries RPC) from the leader.
// It resets the election timer by updating internal state and returns
// whether the heartbeat was accepted.
func (rn *RaftNode) ReceiveHeartbeat(ctx context.Context, leaderID string, leaderTerm uint64, entries []*LogEntry) (uint64, bool) {
	rn.mu.Lock()
	defer rn.mu.Unlock()

	if leaderTerm < rn.currentTerm {
		return rn.currentTerm, false
	}

	if leaderTerm > rn.currentTerm {
		rn.currentTerm = leaderTerm
		rn.metrics.TermChanges++
	}

	rn.state = StateFollower
	rn.leader = leaderID
	rn.votedFor = ""

	// Append any new entries
	for _, entry := range entries {
		existing := rn.getEntry(entry.Index)
		if existing == nil {
			rn.log = append(rn.log, entry)
		} else if existing.Term != entry.Term {
			// Conflict: remove existing entry and all after it
			rn.truncateLogFrom(entry.Index)
			rn.log = append(rn.log, entry)
		}
	}

	return rn.currentTerm, true
}

// truncateLogFrom removes all log entries at or after the given index.
// Must be called with rn.mu held.
func (rn *RaftNode) truncateLogFrom(index uint64) {
	filtered := make([]*LogEntry, 0, len(rn.log))
	for _, entry := range rn.log {
		if entry.Index < index {
			filtered = append(filtered, entry)
		}
	}
	rn.log = filtered
}

// SetPeers updates the set of known peers.
func (rn *RaftNode) SetPeers(ctx context.Context, peerIDs []string) {
	rn.mu.Lock()
	defer rn.mu.Unlock()

	newPeers := make(map[string]*PeerState, len(peerIDs))
	for _, id := range peerIDs {
		if existing, ok := rn.peers[id]; ok {
			newPeers[id] = existing
		} else {
			newPeers[id] = &PeerState{
				ID:        id,
				NextIndex: rn.lastLogIndex() + 1,
			}
		}
	}
	rn.peers = newPeers
}

// AddPeer adds a single peer to the cluster dynamically.
func (rn *RaftNode) AddPeer(ctx context.Context, peerID string) error {
	rn.mu.Lock()
	defer rn.mu.Unlock()

	if peerID == "" {
		return fmt.Errorf("adding peer: peer ID is required")
	}
	if peerID == rn.config.NodeID {
		return fmt.Errorf("adding peer: cannot add self as peer")
	}
	if _, exists := rn.peers[peerID]; exists {
		return fmt.Errorf("adding peer: peer %q already exists", peerID)
	}

	rn.peers[peerID] = &PeerState{
		ID:        peerID,
		NextIndex: rn.lastLogIndex() + 1,
	}
	return nil
}

// RemovePeer removes a single peer from the cluster dynamically.
func (rn *RaftNode) RemovePeer(ctx context.Context, peerID string) error {
	rn.mu.Lock()
	defer rn.mu.Unlock()

	if _, exists := rn.peers[peerID]; !exists {
		return fmt.Errorf("removing peer: peer %q not found", peerID)
	}
	delete(rn.peers, peerID)
	return nil
}

// ListPeers returns the current set of peers and their state.
func (rn *RaftNode) ListPeers(ctx context.Context) []*PeerState {
	rn.mu.RLock()
	defer rn.mu.RUnlock()

	result := make([]*PeerState, 0, len(rn.peers))
	for _, p := range rn.peers {
		result = append(result, p)
	}
	return result
}

// ClusterHealth represents the health status of the Raft cluster.
type ClusterHealth struct {
	Healthy       bool               `json:"healthy"`
	NodeID        string             `json:"node_id"`
	State         RaftState          `json:"state"`
	Term          uint64             `json:"term"`
	Leader        string             `json:"leader"`
	PeerCount     int                `json:"peer_count"`
	CommitIndex   uint64             `json:"commit_index"`
	LastApplied   uint64             `json:"last_applied"`
	LogLength     int                `json:"log_length"`
	HasQuorum     bool               `json:"has_quorum"`
	PeerStatuses  []PeerHealthStatus `json:"peer_statuses"`
}

// PeerHealthStatus represents the health of an individual peer.
type PeerHealthStatus struct {
	ID         string `json:"id"`
	MatchIndex uint64 `json:"match_index"`
	NextIndex  uint64 `json:"next_index"`
}

// GetClusterHealth returns the health status of the cluster.
func (rn *RaftNode) GetClusterHealth(ctx context.Context) *ClusterHealth {
	rn.mu.RLock()
	defer rn.mu.RUnlock()

	peerStatuses := make([]PeerHealthStatus, 0, len(rn.peers))
	for _, p := range rn.peers {
		peerStatuses = append(peerStatuses, PeerHealthStatus{
			ID:         p.ID,
			MatchIndex: p.MatchIndex,
			NextIndex:  p.NextIndex,
		})
	}

	totalNodes := len(rn.peers) + 1 // peers + self
	quorum := totalNodes/2 + 1
	hasQuorum := totalNodes >= quorum

	return &ClusterHealth{
		Healthy:      rn.state != StateFollower || rn.leader != "",
		NodeID:       rn.config.NodeID,
		State:        rn.state,
		Term:         rn.currentTerm,
		Leader:       rn.leader,
		PeerCount:    len(rn.peers),
		CommitIndex:  rn.commitIndex,
		LastApplied:  rn.lastApplied,
		LogLength:    len(rn.log),
		HasQuorum:    hasQuorum,
		PeerStatuses: peerStatuses,
	}
}

// marshalLogEntry serializes a LogEntry to JSON.
func marshalLogEntry(entry *LogEntry) ([]byte, error) {
	data, err := json.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf("marshaling log entry: %w", err)
	}
	return data, nil
}

// unmarshalLogEntry deserializes a LogEntry from JSON.
func unmarshalLogEntry(data []byte) (*LogEntry, error) {
	var entry LogEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("unmarshaling log entry: %w", err)
	}
	return &entry, nil
}

// Package sharding provides distributed sharding and replication
// for the Feather feature store, routing reads and writes through
// a consistent hash ring with configurable replication factors.
package sharding

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/feather-store/feather/internal/platform/cluster"
)

// ReadConsistency controls how reads are routed across replicas.
type ReadConsistency string

const (
	// ReadConsistencyLocal reads from the local node only.
	ReadConsistencyLocal ReadConsistency = "local"
	// ReadConsistencyQuorum reads from a quorum of replicas.
	ReadConsistencyQuorum ReadConsistency = "quorum"
	// ReadConsistencyAll reads from all replicas.
	ReadConsistencyAll ReadConsistency = "all"
)

// WriteConsistency controls how writes are acknowledged across replicas.
type WriteConsistency string

const (
	// WriteConsistencyOne acknowledges after one replica writes.
	WriteConsistencyOne WriteConsistency = "one"
	// WriteConsistencyQuorum acknowledges after a quorum writes.
	WriteConsistencyQuorum WriteConsistency = "quorum"
	// WriteConsistencyAll acknowledges after all replicas write.
	WriteConsistencyAll WriteConsistency = "all"
)

// RouterConfig configures the shard router.
type RouterConfig struct {
	LocalNodeID       string
	ReplicationFactor int
	TotalPartitions   int
	ReadConsistency   ReadConsistency
	WriteConsistency  WriteConsistency
	WriteTimeout      time.Duration
	ReadTimeout       time.Duration
	MaxRetries        int
	ZoneAwareRouting  bool
}

// DefaultRouterConfig returns sensible defaults.
func DefaultRouterConfig() RouterConfig {
	return RouterConfig{
		ReplicationFactor: 3,
		TotalPartitions:   64,
		ReadConsistency:   ReadConsistencyLocal,
		WriteConsistency:  WriteConsistencyQuorum,
		WriteTimeout:      5 * time.Second,
		ReadTimeout:       2 * time.Second,
		MaxRetries:        2,
		ZoneAwareRouting:  true,
	}
}

// ReplicaClient is implemented by node-to-node transport layers.
type ReplicaClient interface {
	WriteFeature(ctx context.Context, nodeID string, req *WriteRequest) error
	ReadFeature(ctx context.Context, nodeID string, req *ReadRequest) (*ReadResponse, error)
}

// WriteRequest describes a feature write to be routed.
type WriteRequest struct {
	EntityKey  string      `json:"entity_key"`
	FeatureKey string      `json:"feature_key"`
	Value      interface{} `json:"value"`
	Timestamp  int64       `json:"timestamp"`
	Version    int64       `json:"version"`
}

// ReadRequest describes a feature read to be routed.
type ReadRequest struct {
	EntityKey   string   `json:"entity_key"`
	FeatureKeys []string `json:"feature_keys"`
}

// ReadResponse wraps a read result from a replica.
type ReadResponse struct {
	NodeID   string                   `json:"node_id"`
	Features map[string]FeatureResult `json:"features"`
}

// FeatureResult holds a single feature value from a replica.
type FeatureResult struct {
	Value     interface{} `json:"value"`
	Timestamp int64       `json:"timestamp"`
	Version   int64       `json:"version"`
	Found     bool        `json:"found"`
}

// RouterStats tracks routing statistics.
type RouterStats struct {
	TotalReads       atomic.Int64
	TotalWrites      atomic.Int64
	LocalReads       atomic.Int64
	RemoteReads      atomic.Int64
	ReplicatedWrites atomic.Int64
	FailedWrites     atomic.Int64
	FailedReads      atomic.Int64
	QuorumFailures   atomic.Int64
}

// Snapshot returns a point-in-time copy of stats.
func (s *RouterStats) Snapshot() map[string]int64 {
	return map[string]int64{
		"total_reads":       s.TotalReads.Load(),
		"total_writes":      s.TotalWrites.Load(),
		"local_reads":       s.LocalReads.Load(),
		"remote_reads":      s.RemoteReads.Load(),
		"replicated_writes": s.ReplicatedWrites.Load(),
		"failed_writes":     s.FailedWrites.Load(),
		"failed_reads":      s.FailedReads.Load(),
		"quorum_failures":   s.QuorumFailures.Load(),
	}
}

// Router routes feature reads and writes across sharded nodes.
type Router struct {
	config       RouterConfig
	ring         *cluster.HashRing
	partitionMap *cluster.PartitionMap
	client       ReplicaClient
	stats        RouterStats
}

// NewRouter creates a new shard router.
func NewRouter(cfg RouterConfig, ring *cluster.HashRing, client ReplicaClient) *Router {
	pm := cluster.NewPartitionMap(ring, cfg.TotalPartitions, cfg.ReplicationFactor)
	return &Router{
		config:       cfg,
		ring:         ring,
		partitionMap: pm,
		client:       client,
	}
}

// RouteWrite sends a write to all replica nodes and waits for the
// configured write consistency level before returning.
func (r *Router) RouteWrite(ctx context.Context, req *WriteRequest) error {
	r.stats.TotalWrites.Add(1)

	owners := r.partitionMap.GetOwnersForKey(req.EntityKey)
	if len(owners) == 0 {
		r.stats.FailedWrites.Add(1)
		return fmt.Errorf("no owners for key %q", req.EntityKey)
	}

	required := r.requiredAcks(len(owners), r.config.WriteConsistency)

	ctx, cancel := context.WithTimeout(ctx, r.config.WriteTimeout)
	defer cancel()

	type ackResult struct {
		nodeID string
		err    error
	}

	results := make(chan ackResult, len(owners))
	for _, node := range owners {
		go func(n *cluster.Node) {
			if n.ID == r.config.LocalNodeID {
				results <- ackResult{nodeID: n.ID, err: nil}
				return
			}
			err := r.client.WriteFeature(ctx, n.ID, req)
			results <- ackResult{nodeID: n.ID, err: err}
		}(node)
	}

	acks := 0
	var lastErr error
	for i := 0; i < len(owners); i++ {
		select {
		case res := <-results:
			if res.err == nil {
				acks++
				r.stats.ReplicatedWrites.Add(1)
			} else {
				lastErr = res.err
			}
		case <-ctx.Done():
			r.stats.FailedWrites.Add(1)
			r.stats.QuorumFailures.Add(1)
			return fmt.Errorf("write timeout: got %d/%d acks: %w", acks, required, ctx.Err())
		}
		if acks >= required {
			return nil
		}
	}

	if acks < required {
		r.stats.FailedWrites.Add(1)
		r.stats.QuorumFailures.Add(1)
		return fmt.Errorf("quorum not reached: got %d/%d acks, last error: %w", acks, required, lastErr)
	}
	return nil
}

// RouteRead routes a read according to the configured consistency.
func (r *Router) RouteRead(ctx context.Context, req *ReadRequest) (*ReadResponse, error) {
	r.stats.TotalReads.Add(1)

	owners := r.partitionMap.GetOwnersForKey(req.EntityKey)
	if len(owners) == 0 {
		r.stats.FailedReads.Add(1)
		return nil, fmt.Errorf("no owners for key %q", req.EntityKey)
	}

	switch r.config.ReadConsistency {
	case ReadConsistencyQuorum:
		return r.readQuorum(ctx, req, owners)
	case ReadConsistencyAll:
		return r.readAll(ctx, req, owners)
	default:
		return r.readLocal(ctx, req, owners)
	}
}

func (r *Router) readLocal(ctx context.Context, req *ReadRequest, owners []*cluster.Node) (*ReadResponse, error) {
	for _, n := range owners {
		if n.ID == r.config.LocalNodeID {
			r.stats.LocalReads.Add(1)
			return &ReadResponse{NodeID: n.ID, Features: make(map[string]FeatureResult)}, nil
		}
	}
	r.stats.RemoteReads.Add(1)
	resp, err := r.client.ReadFeature(ctx, owners[0].ID, req)
	if err != nil {
		r.stats.FailedReads.Add(1)
	}
	return resp, err
}

func (r *Router) readQuorum(ctx context.Context, req *ReadRequest, owners []*cluster.Node) (*ReadResponse, error) {
	required := r.requiredAcks(len(owners), WriteConsistencyQuorum)

	ctx, cancel := context.WithTimeout(ctx, r.config.ReadTimeout)
	defer cancel()

	type readResult struct {
		resp *ReadResponse
		err  error
	}

	results := make(chan readResult, len(owners))
	for _, node := range owners {
		go func(n *cluster.Node) {
			if n.ID == r.config.LocalNodeID {
				r.stats.LocalReads.Add(1)
				results <- readResult{resp: &ReadResponse{NodeID: n.ID, Features: make(map[string]FeatureResult)}}
				return
			}
			r.stats.RemoteReads.Add(1)
			resp, err := r.client.ReadFeature(ctx, n.ID, req)
			results <- readResult{resp: resp, err: err}
		}(node)
	}

	var responses []*ReadResponse
	for i := 0; i < len(owners) && len(responses) < required; i++ {
		select {
		case res := <-results:
			if res.err == nil {
				responses = append(responses, res.resp)
			}
		case <-ctx.Done():
			r.stats.FailedReads.Add(1)
			return nil, fmt.Errorf("read quorum timeout: got %d/%d responses", len(responses), required)
		}
	}

	if len(responses) < required {
		r.stats.FailedReads.Add(1)
		return nil, fmt.Errorf("read quorum not reached: got %d/%d", len(responses), required)
	}
	return r.resolveConflicts(responses), nil
}

func (r *Router) readAll(ctx context.Context, req *ReadRequest, owners []*cluster.Node) (*ReadResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, r.config.ReadTimeout)
	defer cancel()

	type readResult struct {
		resp *ReadResponse
		err  error
	}

	results := make(chan readResult, len(owners))
	for _, node := range owners {
		go func(n *cluster.Node) {
			if n.ID == r.config.LocalNodeID {
				r.stats.LocalReads.Add(1)
				results <- readResult{resp: &ReadResponse{NodeID: n.ID, Features: make(map[string]FeatureResult)}}
				return
			}
			r.stats.RemoteReads.Add(1)
			resp, err := r.client.ReadFeature(ctx, n.ID, req)
			results <- readResult{resp: resp, err: err}
		}(node)
	}

	var responses []*ReadResponse
	for i := 0; i < len(owners); i++ {
		select {
		case res := <-results:
			if res.err == nil {
				responses = append(responses, res.resp)
			}
		case <-ctx.Done():
			break
		}
	}

	if len(responses) == 0 {
		r.stats.FailedReads.Add(1)
		return nil, fmt.Errorf("no successful reads from %d replicas", len(owners))
	}
	return r.resolveConflicts(responses), nil
}

// resolveConflicts picks the newest value for each feature across replicas.
func (r *Router) resolveConflicts(responses []*ReadResponse) *ReadResponse {
	merged := &ReadResponse{Features: make(map[string]FeatureResult)}
	for _, resp := range responses {
		for key, feat := range resp.Features {
			existing, ok := merged.Features[key]
			if !ok || feat.Timestamp > existing.Timestamp {
				merged.Features[key] = feat
				merged.NodeID = resp.NodeID
			}
		}
	}
	return merged
}

// GetPartitionForKey returns the partition owning a key.
func (r *Router) GetPartitionForKey(key string) int {
	return r.partitionMap.GetPartitionForKey(key)
}

// GetOwnersForKey returns the nodes owning a key.
func (r *Router) GetOwnersForKey(key string) []*cluster.Node {
	return r.partitionMap.GetOwnersForKey(key)
}

// IsLocalKey returns true if the local node owns this key.
func (r *Router) IsLocalKey(key string) bool {
	owners := r.partitionMap.GetOwnersForKey(key)
	for _, n := range owners {
		if n.ID == r.config.LocalNodeID {
			return true
		}
	}
	return false
}

// Recompute rebuilds the partition map after topology changes.
func (r *Router) Recompute() {
	r.partitionMap.Recompute()
}

// Stats returns current routing statistics.
func (r *Router) Stats() map[string]int64 {
	return r.stats.Snapshot()
}

// PartitionMap returns the underlying partition map.
func (r *Router) PartitionMap() *cluster.PartitionMap {
	return r.partitionMap
}

func (r *Router) requiredAcks(total int, consistency WriteConsistency) int {
	switch consistency {
	case "one":
		return 1
	case "quorum":
		return total/2 + 1
	case "all":
		return total
	default:
		return total/2 + 1
	}
}

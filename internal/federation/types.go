package federation

import "time"

// NodeRole represents the role of a node in the federation.
type NodeRole string

// NodeRole constants.
const (
	NodeRoleLeader   NodeRole = "leader"
	NodeRoleFollower NodeRole = "follower"
	NodeRolePeer     NodeRole = "peer"
)

// NodeState represents the state of a federated node.
type NodeState string

// NodeState constants.
const (
	NodeStateHealthy     NodeState = "healthy"
	NodeStateUnhealthy   NodeState = "unhealthy"
	NodeStateUnreachable NodeState = "unreachable"
	NodeStateSyncing     NodeState = "syncing"
)

// Node represents a federated feature store node.
type Node struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Address     string            `json:"address"`
	Role        NodeRole          `json:"role"`
	State       NodeState         `json:"state"`
	Region      string            `json:"region"`
	Tags        []string          `json:"tags"`
	Metadata    map[string]string `json:"metadata"`
	JoinedAt    time.Time         `json:"joined_at"`
	LastSeen    time.Time         `json:"last_seen"`
	Version     string            `json:"version"`
	Permissions *NodePermissions  `json:"permissions"`
}

// NodePermissions defines what a node can do in the federation.
type NodePermissions struct {
	CanRead      bool     `json:"can_read"`
	CanWrite     bool     `json:"can_write"`
	CanReplicate bool     `json:"can_replicate"`
	AllowedTeams []string `json:"allowed_teams"`
	DeniedTeams  []string `json:"denied_teams"`
}

// FederatedFeature represents a feature shared across the federation.
type FederatedFeature struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	OwnerNode     string            `json:"owner_node"`
	OwnerTeam     string            `json:"owner_team"`
	DataType      string            `json:"data_type"`
	Tags          []string          `json:"tags"`
	Visibility    Visibility        `json:"visibility"`
	AccessControl *AccessControl    `json:"access_control"`
	Metadata      map[string]string `json:"metadata"`
	Version       int64             `json:"version"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	ReplicatedTo  []string          `json:"replicated_to"`
}

// Visibility defines who can see and access a feature.
type Visibility string

// Visibility constants.
const (
	VisibilityPrivate    Visibility = "private"
	VisibilityTeam       Visibility = "team"
	VisibilityOrg        Visibility = "org"
	VisibilityFederation Visibility = "federation"
	VisibilityPublic     Visibility = "public"
)

// AccessControl defines fine-grained access control for features.
type AccessControl struct {
	AllowedNodes []string       `json:"allowed_nodes"`
	AllowedTeams []string       `json:"allowed_teams"`
	AllowedUsers []string       `json:"allowed_users"`
	DeniedNodes  []string       `json:"denied_nodes"`
	DeniedTeams  []string       `json:"denied_teams"`
	DeniedUsers  []string       `json:"denied_users"`
	RequireAuth  bool           `json:"require_auth"`
	RateLimits   map[string]int `json:"rate_limits"`
}

// CatalogEntry represents a feature in the global catalog.
type CatalogEntry struct {
	Feature      *FederatedFeature `json:"feature"`
	SourceNode   string            `json:"source_node"`
	LocalCopy    bool              `json:"local_copy"`
	CacheExpiry  time.Time         `json:"cache_expiry"`
	AccessCount  int64             `json:"access_count"`
	LastAccessed time.Time         `json:"last_accessed"`
}

// ReplicationPolicy defines how features are replicated.
type ReplicationPolicy struct {
	Mode               ReplicationMode `json:"mode"`
	TargetNodes        []string        `json:"target_nodes"`
	TargetRegions      []string        `json:"target_regions"`
	MinReplicas        int             `json:"min_replicas"`
	MaxReplicas        int             `json:"max_replicas"`
	SyncInterval       time.Duration   `json:"sync_interval"`
	ConflictResolution string          `json:"conflict_resolution"`
}

// ReplicationMode defines how replication works.
type ReplicationMode string

// ReplicationMode constants.
const (
	ReplicationModeSync     ReplicationMode = "sync"
	ReplicationModeAsync    ReplicationMode = "async"
	ReplicationModeOnDemand ReplicationMode = "on_demand"
)

// Config holds federation configuration.
type Config struct {
	NodeID              string
	NodeName            string
	NodeAddress         string
	Region              string
	HealthCheckInterval time.Duration
	SyncInterval        time.Duration
	RequestTimeout      time.Duration
	MaxRetries          int
}

// DefaultConfig returns default federation configuration.
func DefaultConfig() Config {
	return Config{
		HealthCheckInterval: 10 * time.Second,
		SyncInterval:        30 * time.Second,
		RequestTimeout:      5 * time.Second,
		MaxRetries:          3,
	}
}

// EventHandler handles federation events.
type EventHandler func(event Event)

// Event represents a federation event.
type Event struct {
	Type      EventType              `json:"type"`
	NodeID    string                 `json:"node_id"`
	FeatureID string                 `json:"feature_id,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// EventType defines types of federation events.
type EventType string

// EventType constants.
const (
	EventNodeJoined      EventType = "node_joined"
	EventNodeLeft        EventType = "node_left"
	EventNodeUnhealthy   EventType = "node_unhealthy"
	EventFeatureShared   EventType = "feature_shared"
	EventFeatureUpdated  EventType = "feature_updated"
	EventFeatureDeleted  EventType = "feature_deleted"
	EventReplicationDone EventType = "replication_done"
	EventSyncCompleted   EventType = "sync_completed"
)

// SearchQuery defines search parameters for features.
type SearchQuery struct {
	Name       string       `json:"name"`
	Tags       []string     `json:"tags"`
	Teams      []string     `json:"teams"`
	Regions    []string     `json:"regions"`
	Visibility []Visibility `json:"visibility"`
	DataTypes  []string     `json:"data_types"`
	Limit      int          `json:"limit"`
}

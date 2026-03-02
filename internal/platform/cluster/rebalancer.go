package cluster

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// RebalanceState represents the state of a rebalance operation.
type RebalanceState string

const (
	// RebalanceStatePending indicates a pending rebalance.
	RebalanceStatePending RebalanceState = "pending"
	// RebalanceStateRunning indicates a rebalance in progress.
	RebalanceStateRunning RebalanceState = "running"
	// RebalanceStateCompleted indicates a successful rebalance.
	RebalanceStateCompleted RebalanceState = "completed"
	// RebalanceStateFailed indicates a failed rebalance.
	RebalanceStateFailed RebalanceState = "failed"
	// RebalanceStateCancelled indicates a canceled rebalance.
	RebalanceStateCancelled RebalanceState = "cancelled" //nolint:misspell
)

// RebalanceReason represents why a rebalance was triggered.
type RebalanceReason string

const (
	// RebalanceReasonNodeJoin indicates a rebalance from a node join.
	RebalanceReasonNodeJoin RebalanceReason = "node_join"
	// RebalanceReasonNodeLeave indicates a rebalance from a node leave.
	RebalanceReasonNodeLeave RebalanceReason = "node_leave"
	// RebalanceReasonManual indicates an operator-triggered rebalance.
	RebalanceReasonManual RebalanceReason = "manual"
	// RebalanceReasonScheduled indicates a scheduled rebalance.
	RebalanceReasonScheduled RebalanceReason = "scheduled"
	// RebalanceReasonImbalance indicates a rebalance due to imbalance.
	RebalanceReasonImbalance RebalanceReason = "imbalance"
)

// RebalanceTask represents a single partition transfer task.
type RebalanceTask struct {
	ID          string         `json:"id"`
	Partition   int            `json:"partition"`
	FromNode    string         `json:"from_node"`
	ToNode      string         `json:"to_node"`
	State       RebalanceState `json:"state"`
	BytesMoved  int64          `json:"bytes_moved"`
	KeysMoved   int64          `json:"keys_moved"`
	StartedAt   time.Time      `json:"started_at,omitempty"`
	CompletedAt time.Time      `json:"completed_at,omitempty"`
	Error       string         `json:"error,omitempty"`
	Progress    float64        `json:"progress"`
}

// RebalancePlan represents a complete rebalance plan.
type RebalancePlan struct {
	ID          string           `json:"id"`
	Reason      RebalanceReason  `json:"reason"`
	State       RebalanceState   `json:"state"`
	Tasks       []*RebalanceTask `json:"tasks"`
	CreatedAt   time.Time        `json:"created_at"`
	StartedAt   time.Time        `json:"started_at,omitempty"`
	CompletedAt time.Time        `json:"completed_at,omitempty"`
	TotalBytes  int64            `json:"total_bytes"`
	TotalKeys   int64            `json:"total_keys"`
}

// Progress returns the overall progress of the rebalance plan.
func (p *RebalancePlan) Progress() float64 {
	if len(p.Tasks) == 0 {
		return 1.0
	}

	completed := 0
	for _, task := range p.Tasks {
		if task.State == RebalanceStateCompleted {
			completed++
		}
	}
	return float64(completed) / float64(len(p.Tasks))
}

// RebalancerConfig configures the rebalancer.
type RebalancerConfig struct {
	MaxConcurrentTasks int
	TaskTimeout        time.Duration
	CheckInterval      time.Duration
	ImbalanceThreshold float64 // Percentage threshold to trigger rebalance
	MinRebalanceDelay  time.Duration
	DryRun             bool
}

// DefaultRebalancerConfig returns sensible defaults.
func DefaultRebalancerConfig() RebalancerConfig {
	return RebalancerConfig{
		MaxConcurrentTasks: 4,
		TaskTimeout:        5 * time.Minute,
		CheckInterval:      30 * time.Second,
		ImbalanceThreshold: 0.1, // 10% imbalance
		MinRebalanceDelay:  1 * time.Minute,
		DryRun:             false,
	}
}

// DataTransferFunc is called to transfer data for a partition.
type DataTransferFunc func(ctx context.Context, task *RebalanceTask) error

// Rebalancer manages partition rebalancing across the cluster.
type Rebalancer struct {
	config       RebalancerConfig
	membership   *MembershipManager
	ring         *HashRing
	partitionMap *PartitionMap
	transferFunc DataTransferFunc

	currentPlan   *RebalancePlan
	planHistory   []*RebalancePlan
	lastRebalance time.Time
	mu            sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewRebalancer creates a new rebalancer.
func NewRebalancer(ctx context.Context, config RebalancerConfig, membership *MembershipManager, ring *HashRing, partitionMap *PartitionMap) *Rebalancer {
	ctx, cancel := context.WithCancel(ctx)

	r := &Rebalancer{
		config:       config,
		membership:   membership,
		ring:         ring,
		partitionMap: partitionMap,
		planHistory:  make([]*RebalancePlan, 0),
		ctx:          ctx,
		cancel:       cancel,
	}

	return r
}

// SetTransferFunc sets the function used to transfer partition data.
func (r *Rebalancer) SetTransferFunc(fn DataTransferFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.transferFunc = fn
}

// Start begins the rebalancer's background monitoring.
func (r *Rebalancer) Start() error {
	// Register for membership changes
	r.membership.AddListener(r)

	// Start monitoring loop
	r.wg.Add(1)
	go r.monitorLoop()

	return nil
}

// Stop shuts down the rebalancer.
func (r *Rebalancer) Stop() error {
	r.membership.RemoveListener(r)
	r.cancel()
	r.wg.Wait()
	return nil
}

// OnMembershipChange handles membership events.
func (r *Rebalancer) OnMembershipChange(event MembershipEvent) {
	switch event.Type {
	case EventNodeJoin:
		r.onNodeJoin(event.Node)
	case EventNodeLeave, EventNodeDead:
		r.onNodeLeave(event.Node)
	case EventNodeUpdate, EventNodeSuspect, EventNodeAlive:
		return
	}
}

func (r *Rebalancer) onNodeJoin(node *Node) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Add to ring
	r.ring.AddNode(node)

	// Schedule rebalance if needed
	r.scheduleRebalance(RebalanceReasonNodeJoin)
}

func (r *Rebalancer) onNodeLeave(node *Node) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove from ring
	r.ring.RemoveNode(node.ID)

	// Schedule rebalance
	r.scheduleRebalance(RebalanceReasonNodeLeave)
}

func (r *Rebalancer) scheduleRebalance(reason RebalanceReason) {
	// Check if we're already rebalancing
	if r.currentPlan != nil && r.currentPlan.State == RebalanceStateRunning {
		return
	}

	// Check minimum delay
	if time.Since(r.lastRebalance) < r.config.MinRebalanceDelay {
		return
	}

	// Create rebalance plan
	plan := r.createRebalancePlan(reason)
	if plan == nil || len(plan.Tasks) == 0 {
		return
	}

	r.currentPlan = plan
}

// TriggerRebalance manually triggers a rebalance.
func (r *Rebalancer) TriggerRebalance() (*RebalancePlan, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.currentPlan != nil && r.currentPlan.State == RebalanceStateRunning {
		return nil, fmt.Errorf("rebalance already in progress")
	}

	plan := r.createRebalancePlan(RebalanceReasonManual)
	if plan == nil || len(plan.Tasks) == 0 {
		return nil, fmt.Errorf("no rebalancing needed")
	}

	r.currentPlan = plan
	return plan, nil
}

// GetCurrentPlan returns the current rebalance plan if any.
func (r *Rebalancer) GetCurrentPlan() *RebalancePlan {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.currentPlan
}

// GetPlanHistory returns completed rebalance plans.
func (r *Rebalancer) GetPlanHistory() []*RebalancePlan {
	r.mu.RLock()
	defer r.mu.RUnlock()

	history := make([]*RebalancePlan, len(r.planHistory))
	copy(history, r.planHistory)
	return history
}

// CancelRebalance cancels the current rebalance operation.
func (r *Rebalancer) CancelRebalance() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.currentPlan == nil || r.currentPlan.State != RebalanceStateRunning {
		return fmt.Errorf("no rebalance in progress")
	}

	r.currentPlan.State = RebalanceStateCancelled
	r.currentPlan.CompletedAt = time.Now()

	r.planHistory = append(r.planHistory, r.currentPlan)
	r.currentPlan = nil

	return nil
}

func (r *Rebalancer) createRebalancePlan(reason RebalanceReason) *RebalancePlan {
	// Get current distribution
	oldDistribution := make(map[int][]*Node)
	for p := 0; p < r.partitionMap.TotalPartitions(); p++ {
		oldDistribution[p] = r.partitionMap.GetOwners(p)
	}

	// Recompute partition map
	r.partitionMap.Recompute()

	// Get new distribution
	newDistribution := make(map[int][]*Node)
	for p := 0; p < r.partitionMap.TotalPartitions(); p++ {
		newDistribution[p] = r.partitionMap.GetOwners(p)
	}

	// Calculate tasks
	var tasks []*RebalanceTask
	for partition := 0; partition < r.partitionMap.TotalPartitions(); partition++ {
		oldOwners := nodeIDSet(oldDistribution[partition])
		newOwners := nodeIDSet(newDistribution[partition])

		// Find nodes that need to receive data
		for newOwnerID := range newOwners {
			if !oldOwners[newOwnerID] {
				// Find a source node
				var fromNode string
				for oldOwnerID := range oldOwners {
					fromNode = oldOwnerID
					break
				}

				if fromNode != "" {
					tasks = append(tasks, &RebalanceTask{
						ID:        uuid.New().String(),
						Partition: partition,
						FromNode:  fromNode,
						ToNode:    newOwnerID,
						State:     RebalanceStatePending,
					})
				}
			}
		}
	}

	if len(tasks) == 0 {
		return nil
	}

	return &RebalancePlan{
		ID:        uuid.New().String(),
		Reason:    reason,
		State:     RebalanceStatePending,
		Tasks:     tasks,
		CreatedAt: time.Now(),
	}
}

func nodeIDSet(nodes []*Node) map[string]bool {
	set := make(map[string]bool)
	for _, n := range nodes {
		if n != nil {
			set[n.ID] = true
		}
	}
	return set
}

func (r *Rebalancer) monitorLoop() {
	defer r.wg.Done()

	ticker := time.NewTicker(r.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.checkAndExecute()
		}
	}
}

func (r *Rebalancer) checkAndExecute() {
	r.mu.Lock()

	// Check if we should auto-rebalance
	if r.currentPlan == nil {
		if r.shouldAutoRebalance() {
			r.scheduleRebalance(RebalanceReasonImbalance)
		}
	}

	plan := r.currentPlan
	if plan == nil || plan.State != RebalanceStatePending {
		r.mu.Unlock()
		return
	}

	// Start the plan
	plan.State = RebalanceStateRunning
	plan.StartedAt = time.Now()
	r.lastRebalance = time.Now()

	r.mu.Unlock()

	// Execute the plan
	r.executePlan(plan)
}

func (r *Rebalancer) shouldAutoRebalance() bool {
	distribution := r.partitionMap.GetPartitionDistribution()
	if len(distribution) < 2 {
		return false
	}

	// Calculate standard deviation of partition counts
	var total float64
	for _, count := range distribution {
		total += float64(count)
	}
	mean := total / float64(len(distribution))

	var variance float64
	for _, count := range distribution {
		diff := float64(count) - mean
		variance += diff * diff
	}
	variance /= float64(len(distribution))

	// Calculate coefficient of variation
	if mean == 0 {
		return false
	}
	cv := variance / (mean * mean)

	return cv > r.config.ImbalanceThreshold
}

func (r *Rebalancer) executePlan(plan *RebalancePlan) {
	if r.config.DryRun {
		r.mu.Lock()
		plan.State = RebalanceStateCompleted
		plan.CompletedAt = time.Now()
		for _, task := range plan.Tasks {
			task.State = RebalanceStateCompleted
		}
		r.planHistory = append(r.planHistory, plan)
		r.currentPlan = nil
		r.mu.Unlock()
		return
	}

	// Execute tasks with limited concurrency
	sem := make(chan struct{}, r.config.MaxConcurrentTasks)
	var wg sync.WaitGroup

	for _, task := range plan.Tasks {
		if plan.State == RebalanceStateCancelled {
			break
		}

		sem <- struct{}{}
		wg.Add(1)

		go func(t *RebalanceTask) {
			defer wg.Done()
			defer func() { <-sem }()

			r.executeTask(t)
		}(task)
	}

	wg.Wait()

	// Finalize plan
	r.mu.Lock()
	defer r.mu.Unlock()

	allCompleted := true
	anyFailed := false
	for _, task := range plan.Tasks {
		if task.State != RebalanceStateCompleted {
			allCompleted = false
		}
		if task.State == RebalanceStateFailed {
			anyFailed = true
		}
	}

	if plan.State != RebalanceStateCancelled {
		if anyFailed {
			plan.State = RebalanceStateFailed
		} else if allCompleted {
			plan.State = RebalanceStateCompleted
		}
	}
	plan.CompletedAt = time.Now()

	r.planHistory = append(r.planHistory, plan)
	r.currentPlan = nil
}

func (r *Rebalancer) executeTask(task *RebalanceTask) {
	r.mu.Lock()
	task.State = RebalanceStateRunning
	task.StartedAt = time.Now()
	transferFunc := r.transferFunc
	r.mu.Unlock()

	ctx, cancel := context.WithTimeout(r.ctx, r.config.TaskTimeout)
	defer cancel()

	var err error
	if transferFunc != nil {
		err = transferFunc(ctx, task)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if err != nil {
		task.State = RebalanceStateFailed
		task.Error = err.Error()
	} else {
		task.State = RebalanceStateCompleted
		task.Progress = 1.0
	}
	task.CompletedAt = time.Now()
}

// RebalancerStats returns statistics about the rebalancer.
type RebalancerStats struct {
	CurrentPlan           *RebalancePlan `json:"current_plan,omitempty"`
	TotalRebalances       int            `json:"total_rebalances"`
	SuccessfulRebalances  int            `json:"successful_rebalances"`
	FailedRebalances      int            `json:"failed_rebalances"`
	LastRebalance         time.Time      `json:"last_rebalance,omitempty"`
	PartitionDistribution map[string]int `json:"partition_distribution"`
}

// Stats returns rebalancer statistics.
func (r *Rebalancer) Stats() RebalancerStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := RebalancerStats{
		CurrentPlan:           r.currentPlan,
		TotalRebalances:       len(r.planHistory),
		LastRebalance:         r.lastRebalance,
		PartitionDistribution: r.partitionMap.GetPartitionDistribution(),
	}

	for _, plan := range r.planHistory {
		if plan.State == RebalanceStateCompleted {
			stats.SuccessfulRebalances++
		} else if plan.State == RebalanceStateFailed {
			stats.FailedRebalances++
		}
	}

	return stats
}

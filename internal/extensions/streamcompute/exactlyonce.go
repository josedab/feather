package streamcompute

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ExactlyOnceConfig configures the exactly-once processor.
type ExactlyOnceConfig struct {
	DeduplicationWindowSize int           `json:"deduplication_window_size"`
	CheckpointInterval      time.Duration `json:"checkpoint_interval"`
	MaxPendingTransactions  int           `json:"max_pending_transactions"`
}

// DefaultExactlyOnceConfig returns sensible defaults.
func DefaultExactlyOnceConfig() ExactlyOnceConfig {
	return ExactlyOnceConfig{
		DeduplicationWindowSize: 10000,
		CheckpointInterval:      30 * time.Second,
		MaxPendingTransactions:  100,
	}
}

// TransactionStatus represents the state of a transaction.
type TransactionStatus string

// TransactionStatus values.
const (
	TxPending    TransactionStatus = "pending"
	TxCommitted  TransactionStatus = "committed"
	TxRolledBack TransactionStatus = "rolled_back"
)

// Transaction represents a group of events processed atomically.
type Transaction struct {
	ID        string            `json:"id"`
	Events    []Event           `json:"events"`
	Status    TransactionStatus `json:"status"`
	StartedAt time.Time         `json:"started_at"`
}

// ExactlyOnceStats provides statistics about exactly-once processing.
type ExactlyOnceStats struct {
	TotalTransactions      int64 `json:"total_transactions"`
	CommittedTransactions  int64 `json:"committed_transactions"`
	RolledBackTransactions int64 `json:"rolled_back_transactions"`
	PendingTransactions    int   `json:"pending_transactions"`
	DuplicatesDetected     int64 `json:"duplicates_detected"`
	DeduplicationSize      int   `json:"deduplication_size"`
}

// ExactlyOnceProcessor provides exactly-once semantics for stream processing
// through transaction-like processing, idempotent deduplication, and
// checkpoint-based recovery.
type ExactlyOnceProcessor struct {
	mu           sync.RWMutex
	config       ExactlyOnceConfig
	transactions map[string]*Transaction
	seenIDs      map[string]time.Time // eventID -> observed time
	txCounter    int64

	totalTx      int64
	committedTx  int64
	rolledBackTx int64
	duplicates   int64
}

// NewExactlyOnceProcessor creates a new exactly-once processor.
func NewExactlyOnceProcessor(cfg ExactlyOnceConfig) *ExactlyOnceProcessor {
	if cfg.DeduplicationWindowSize <= 0 {
		cfg.DeduplicationWindowSize = DefaultExactlyOnceConfig().DeduplicationWindowSize
	}
	if cfg.CheckpointInterval <= 0 {
		cfg.CheckpointInterval = DefaultExactlyOnceConfig().CheckpointInterval
	}
	if cfg.MaxPendingTransactions <= 0 {
		cfg.MaxPendingTransactions = DefaultExactlyOnceConfig().MaxPendingTransactions
	}
	return &ExactlyOnceProcessor{
		config:       cfg,
		transactions: make(map[string]*Transaction),
		seenIDs:      make(map[string]time.Time),
	}
}

// BeginTransaction starts a new transaction.
func (p *ExactlyOnceProcessor) BeginTransaction() (*Transaction, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	pending := 0
	for _, tx := range p.transactions {
		if tx.Status == TxPending {
			pending++
		}
	}
	if pending >= p.config.MaxPendingTransactions {
		return nil, fmt.Errorf("max pending transactions reached (%d)", p.config.MaxPendingTransactions)
	}

	id := fmt.Sprintf("tx-%d", atomic.AddInt64(&p.txCounter, 1))
	tx := &Transaction{
		ID:        id,
		Events:    make([]Event, 0),
		Status:    TxPending,
		StartedAt: time.Now(),
	}
	p.transactions[id] = tx
	atomic.AddInt64(&p.totalTx, 1)
	return tx, nil
}

// ProcessEvent adds an event to a transaction after deduplication.
func (p *ExactlyOnceProcessor) ProcessEvent(tx *Transaction, event Event) error {
	if tx == nil {
		return fmt.Errorf("transaction is nil")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	stored, exists := p.transactions[tx.ID]
	if !exists {
		return fmt.Errorf("transaction %q not found", tx.ID)
	}
	if stored.Status != TxPending {
		return fmt.Errorf("transaction %q is not pending (status: %s)", tx.ID, stored.Status)
	}

	eventID := fmt.Sprintf("%s:%s:%d", event.Key, event.Timestamp.Format(time.RFC3339Nano), int64(event.Value*1000))
	if _, seen := p.seenIDs[eventID]; seen {
		atomic.AddInt64(&p.duplicates, 1)
		return nil // Idempotent: silently skip duplicates
	}

	stored.Events = append(stored.Events, event)
	tx.Events = stored.Events
	return nil
}

// Commit commits a transaction, marking its events as processed.
func (p *ExactlyOnceProcessor) Commit(tx *Transaction) error {
	if tx == nil {
		return fmt.Errorf("transaction is nil")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	stored, exists := p.transactions[tx.ID]
	if !exists {
		return fmt.Errorf("transaction %q not found", tx.ID)
	}
	if stored.Status != TxPending {
		return fmt.Errorf("transaction %q is not pending (status: %s)", tx.ID, stored.Status)
	}

	// Record all event IDs in the deduplication window
	for _, event := range stored.Events {
		eventID := fmt.Sprintf("%s:%s:%d", event.Key, event.Timestamp.Format(time.RFC3339Nano), int64(event.Value*1000))
		p.seenIDs[eventID] = time.Now()
	}

	// Evict oldest entries if deduplication window is exceeded
	p.evictOldEntries()

	stored.Status = TxCommitted
	tx.Status = TxCommitted
	atomic.AddInt64(&p.committedTx, 1)
	return nil
}

// Rollback rolls back a transaction, discarding its events.
func (p *ExactlyOnceProcessor) Rollback(tx *Transaction) error {
	if tx == nil {
		return fmt.Errorf("transaction is nil")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	stored, exists := p.transactions[tx.ID]
	if !exists {
		return fmt.Errorf("transaction %q not found", tx.ID)
	}
	if stored.Status != TxPending {
		return fmt.Errorf("transaction %q is not pending (status: %s)", tx.ID, stored.Status)
	}

	stored.Status = TxRolledBack
	stored.Events = nil
	tx.Status = TxRolledBack
	tx.Events = nil
	atomic.AddInt64(&p.rolledBackTx, 1)
	return nil
}

// IsDuplicate checks if an event ID has already been processed.
func (p *ExactlyOnceProcessor) IsDuplicate(eventID string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, exists := p.seenIDs[eventID]
	return exists
}

// Stats returns exactly-once processing statistics.
func (p *ExactlyOnceProcessor) Stats() ExactlyOnceStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	pending := 0
	for _, tx := range p.transactions {
		if tx.Status == TxPending {
			pending++
		}
	}

	return ExactlyOnceStats{
		TotalTransactions:      atomic.LoadInt64(&p.totalTx),
		CommittedTransactions:  atomic.LoadInt64(&p.committedTx),
		RolledBackTransactions: atomic.LoadInt64(&p.rolledBackTx),
		PendingTransactions:    pending,
		DuplicatesDetected:     atomic.LoadInt64(&p.duplicates),
		DeduplicationSize:      len(p.seenIDs),
	}
}

// evictOldEntries removes the oldest deduplication entries when the window is full.
// Must be called with p.mu held.
func (p *ExactlyOnceProcessor) evictOldEntries() {
	if len(p.seenIDs) <= p.config.DeduplicationWindowSize {
		return
	}

	// Find and remove oldest entries until within limit
	for len(p.seenIDs) > p.config.DeduplicationWindowSize {
		var oldestKey string
		var oldestTime time.Time
		first := true
		for k, t := range p.seenIDs {
			if first || t.Before(oldestTime) {
				oldestKey = k
				oldestTime = t
				first = false
			}
		}
		delete(p.seenIDs, oldestKey)
	}
}

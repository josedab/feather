package tenant

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

//revive:disable:exported

// Middleware provides HTTP middleware for multi-tenant operations.
type Middleware struct {
	registry *TenantRegistry
}

// NewTenantMiddleware creates a new tenant middleware.
func NewTenantMiddleware(registry *TenantRegistry) *Middleware {
	return &Middleware{
		registry: registry,
	}
}

// ExtractTenant extracts tenant ID from the request and adds it to context.
func (m *Middleware) ExtractTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try multiple sources for tenant ID
		tenantID := r.Header.Get("X-Tenant-ID")
		if tenantID == "" {
			tenantID = r.URL.Query().Get("tenant_id")
		}
		if tenantID == "" {
			tenantID = "default"
		}

		// Add tenant to context
		ctx := WithTenant(r.Context(), tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// EnforceTenant ensures a valid tenant is present and enabled.
func (m *Middleware) EnforceTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := TenantFromContext(r.Context())
		if tenantID == "" {
			http.Error(w, "tenant ID required", http.StatusUnauthorized)
			return
		}

		tenant, err := m.registry.GetTenant(tenantID)
		if err != nil {
			http.Error(w, "invalid tenant", http.StatusUnauthorized)
			return
		}

		if !tenant.Enabled {
			http.Error(w, "tenant disabled", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// EnforceRateLimit enforces rate limiting for the tenant.
func (m *Middleware) EnforceRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := TenantFromContext(r.Context())
		if tenantID == "" {
			next.ServeHTTP(w, r)
			return
		}

		if err := m.registry.CheckRateLimit(tenantID); err != nil {
			if errors.Is(err, ErrRateLimitExceeded) {
				w.Header().Set("Retry-After", "1")
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			// Other errors - allow request to proceed
		}

		next.ServeHTTP(w, r)
	})
}

// EnforceQuota checks storage quota before write operations.
func (m *Middleware) EnforceQuota(quotaType string, amount int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := TenantFromContext(r.Context())
			if tenantID == "" {
				next.ServeHTTP(w, r)
				return
			}

			if err := m.registry.CheckQuota(tenantID, quotaType, amount); err != nil {
				http.Error(w, err.Error(), http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RecordMetrics records request metrics for the tenant.
func (m *Middleware) RecordMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := TenantFromContext(r.Context())
		start := time.Now()

		// Wrap response writer to capture status
		wrapped := &statusCapture{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		if tenantID != "" {
			latency := time.Since(start)
			isError := wrapped.status >= 400
			m.registry.RecordRequest(tenantID, latency, isError)
		}
	})
}

//revive:enable:exported

// statusCapture captures the HTTP status code.
type statusCapture struct {
	http.ResponseWriter
	status int
}

func (s *statusCapture) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// ConcurrencyLimiter limits concurrent requests per tenant.
type ConcurrencyLimiter struct {
	registry *TenantRegistry
	counters sync.Map // tenantID -> *int64
}

// NewConcurrencyLimiter creates a new concurrency limiter.
func NewConcurrencyLimiter(registry *TenantRegistry) *ConcurrencyLimiter {
	return &ConcurrencyLimiter{
		registry: registry,
	}
}

// Limit creates middleware that limits concurrent requests.
func (l *ConcurrencyLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := TenantFromContext(r.Context())
		if tenantID == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Get or create counter
		counterI, _ := l.counters.LoadOrStore(tenantID, new(int64))
		counter, ok := counterI.(*int64)
		if !ok {
			http.Error(w, "concurrency limiter unavailable", http.StatusInternalServerError)
			return
		}

		// Check limit
		if err := l.registry.CheckQuota(tenantID, "concurrent", 1); err != nil {
			http.Error(w, "too many concurrent requests", http.StatusTooManyRequests)
			return
		}

		// Increment counter
		atomic.AddInt64(counter, 1)
		if err := l.registry.UpdateUsage(tenantID, func(u *TenantUsage) {
			atomic.AddInt64(&u.ConcurrentRequests, 1)
		}); err != nil {
			http.Error(w, "tenant not found", http.StatusUnauthorized)
			atomic.AddInt64(counter, -1)
			return
		}

		// Ensure decrement on completion
		defer func() {
			atomic.AddInt64(counter, -1)
			if err := l.registry.UpdateUsage(tenantID, func(u *TenantUsage) {
				atomic.AddInt64(&u.ConcurrentRequests, -1)
			}); err != nil {
				slog.Debug("failed to update tenant usage", "tenant", tenantID, "error", err)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// PriorityQueue manages request prioritization by tenant tier.
type PriorityQueue struct {
	mu       sync.Mutex
	registry *TenantRegistry
	queues   map[PriorityClass]chan *priorityRequest
	workers  int
	running  bool
	stopCh   chan struct{}
}

type priorityRequest struct {
	handler  http.Handler
	writer   http.ResponseWriter
	request  *http.Request
	done     chan struct{}
	priority PriorityClass
}

// NewPriorityQueue creates a new priority queue.
func NewPriorityQueue(registry *TenantRegistry, workers int) *PriorityQueue {
	if workers <= 0 {
		workers = 10
	}

	pq := &PriorityQueue{
		registry: registry,
		workers:  workers,
		queues: map[PriorityClass]chan *priorityRequest{
			PriorityCritical: make(chan *priorityRequest, 100),
			PriorityHigh:     make(chan *priorityRequest, 500),
			PriorityNormal:   make(chan *priorityRequest, 1000),
			PriorityLow:      make(chan *priorityRequest, 500),
		},
		stopCh: make(chan struct{}),
	}

	return pq
}

// Start starts the priority queue workers.
func (pq *PriorityQueue) Start() {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.running {
		return
	}

	pq.running = true

	for i := 0; i < pq.workers; i++ {
		go pq.worker()
	}
}

// Stop stops the priority queue.
func (pq *PriorityQueue) Stop() {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if !pq.running {
		return
	}

	pq.running = false
	close(pq.stopCh)
}

func (pq *PriorityQueue) worker() {
	for {
		select {
		case <-pq.stopCh:
			return
		default:
			// Process requests in priority order
			var req *priorityRequest

			select {
			case req = <-pq.queues[PriorityCritical]:
			default:
				select {
				case req = <-pq.queues[PriorityCritical]:
				case req = <-pq.queues[PriorityHigh]:
				default:
					select {
					case req = <-pq.queues[PriorityCritical]:
					case req = <-pq.queues[PriorityHigh]:
					case req = <-pq.queues[PriorityNormal]:
					default:
						select {
						case req = <-pq.queues[PriorityCritical]:
						case req = <-pq.queues[PriorityHigh]:
						case req = <-pq.queues[PriorityNormal]:
						case req = <-pq.queues[PriorityLow]:
						case <-pq.stopCh:
							return
						}
					}
				}
			}

			if req != nil {
				req.handler.ServeHTTP(req.writer, req.request)
				close(req.done)
			}
		}
	}
}

// Middleware returns middleware that routes requests through the priority queue.
func (pq *PriorityQueue) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := TenantFromContext(r.Context())
		priority := PriorityNormal

		if tenantID != "" {
			priority = pq.registry.GetPriority(tenantID)
		}

		// For critical and high priority, process directly if queue is small
		if priority >= PriorityHigh && len(pq.queues[priority]) < 10 {
			next.ServeHTTP(w, r)
			return
		}

		req := &priorityRequest{
			handler:  next,
			writer:   w,
			request:  r,
			done:     make(chan struct{}),
			priority: priority,
		}

		// Try to enqueue
		select {
		case pq.queues[priority] <- req:
			<-req.done
		default:
			// Queue full, process directly
			next.ServeHTTP(w, r)
		}
	})
}

// AwareStore wraps storage operations with tenant isolation.
type AwareStore struct {
	registry *TenantRegistry
}

// NewTenantAwareStore creates a new tenant-aware store wrapper.
func NewTenantAwareStore(registry *TenantRegistry) *AwareStore {
	return &AwareStore{
		registry: registry,
	}
}

// PrefixEntityKey adds tenant prefix to entity key for isolation.
func (s *AwareStore) PrefixEntityKey(ctx context.Context, entityKey string) string {
	tenantID := TenantFromContext(ctx)
	if tenantID == "" || tenantID == "default" {
		return entityKey
	}
	return tenantID + ":" + entityKey
}

// StripTenantPrefix removes tenant prefix from entity key.
func (s *AwareStore) StripTenantPrefix(ctx context.Context, entityKey string) string {
	tenantID := TenantFromContext(ctx)
	if tenantID == "" || tenantID == "default" {
		return entityKey
	}

	prefix := tenantID + ":"
	if len(entityKey) > len(prefix) && entityKey[:len(prefix)] == prefix {
		return entityKey[len(prefix):]
	}
	return entityKey
}

// CheckWriteAccess checks if the tenant can write to storage.
func (s *AwareStore) CheckWriteAccess(ctx context.Context, estimatedBytes int64) error {
	tenantID := TenantFromContext(ctx)
	if tenantID == "" {
		return nil
	}

	return s.registry.CheckQuota(tenantID, "storage", estimatedBytes)
}

// RecordStorageUsage updates storage usage for a tenant.
func (s *AwareStore) RecordStorageUsage(ctx context.Context, deltaBytes int64) {
	tenantID := TenantFromContext(ctx)
	if tenantID == "" {
		return
	}

	if err := s.registry.UpdateUsage(tenantID, func(u *TenantUsage) {
		u.StorageBytes += deltaBytes
	}); err != nil {
		return
	}
}

// RecordEntityUsage updates entity count for a tenant.
func (s *AwareStore) RecordEntityUsage(ctx context.Context, delta int64) {
	tenantID := TenantFromContext(ctx)
	if tenantID == "" {
		return
	}

	if err := s.registry.UpdateUsage(tenantID, func(u *TenantUsage) {
		u.EntityCount += delta
	}); err != nil {
		return
	}
}

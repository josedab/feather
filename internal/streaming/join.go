package streaming

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// JoinType defines the type of join operation.
type JoinType string

const (
	// JoinTypeInner returns only matching records from both streams.
	JoinTypeInner JoinType = "inner"
	// JoinTypeLeft returns all records from left stream with matching right records.
	JoinTypeLeft JoinType = "left"
	// JoinTypeRight returns all records from right stream with matching left records.
	JoinTypeRight JoinType = "right"
	// JoinTypeFull returns all records from both streams.
	JoinTypeFull JoinType = "full"
)

// JoinConfig configures a streaming join operation.
type JoinConfig struct {
	// Name uniquely identifies this join.
	Name string `json:"name"`

	// Type specifies the join type (inner, left, right, full).
	Type JoinType `json:"type"`

	// LeftStream is the name/type of the left input stream.
	LeftStream string `json:"left_stream"`

	// RightStream is the name/type of the right input stream.
	RightStream string `json:"right_stream"`

	// JoinKey is the field to join on (must exist in both streams).
	JoinKey string `json:"join_key"`

	// LeftKey overrides the join key for left stream (optional).
	LeftKey string `json:"left_key,omitempty"`

	// RightKey overrides the join key for right stream (optional).
	RightKey string `json:"right_key,omitempty"`

	// WindowDuration is the join window duration.
	WindowDuration time.Duration `json:"window_duration"`

	// GracePeriod allows late events within this period.
	GracePeriod time.Duration `json:"grace_period"`

	// OutputFields specifies which fields to include in output.
	OutputFields []JoinOutputField `json:"output_fields"`

	// TimestampField for event-time join (optional, uses processing time if empty).
	TimestampField string `json:"timestamp_field,omitempty"`

	// WatermarkInterval for watermark generation.
	WatermarkInterval time.Duration `json:"watermark_interval"`
}

// JoinOutputField specifies a field to include in join output.
type JoinOutputField struct {
	// Source is "left" or "right".
	Source string `json:"source"`
	// Field is the source field name.
	Field string `json:"field"`
	// Alias is the output field name (optional, defaults to Field).
	Alias string `json:"alias,omitempty"`
}

// JoinOperator performs streaming joins between two event streams.
type JoinOperator struct {
	config     JoinConfig
	leftState  *joinStateStore
	rightState *joinStateStore
	watermark  time.Time
	outputChan chan *JoinResult
	metrics    *JoinMetrics
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

// JoinResult represents the result of a join operation.
type JoinResult struct {
	JoinName    string                 `json:"join_name"`
	JoinKey     string                 `json:"join_key"`
	LeftEvent   *Event                 `json:"left_event,omitempty"`
	RightEvent  *Event                 `json:"right_event,omitempty"`
	OutputData  map[string]interface{} `json:"output_data"`
	EventTime   time.Time              `json:"event_time"`
	ProcessTime time.Time              `json:"process_time"`
	IsLateJoin  bool                   `json:"is_late_join"`
}

// JoinMetrics tracks join operation metrics.
type JoinMetrics struct {
	mu             sync.RWMutex
	LeftEvents     int64     `json:"left_events"`
	RightEvents    int64     `json:"right_events"`
	JoinMatches    int64     `json:"join_matches"`
	LateEvents     int64     `json:"late_events"`
	DroppedEvents  int64     `json:"dropped_events"`
	StateSize      int64     `json:"state_size"`
	LastWatermark  time.Time `json:"last_watermark"`
	AvgJoinLatency float64   `json:"avg_join_latency_ms"`
}

// joinStateStore holds events for one side of the join.
type joinStateStore struct {
	mu      sync.RWMutex
	events  map[string][]*timedEvent // key -> events
	expiry  time.Duration
	maxSize int
}

type timedEvent struct {
	event     *Event
	timestamp time.Time
	processed time.Time
}

// NewJoinOperator creates a new streaming join operator.
func NewJoinOperator(config JoinConfig) *JoinOperator {
	ctx, cancel := context.WithCancel(context.Background())

	if config.WindowDuration == 0 {
		config.WindowDuration = 5 * time.Minute
	}
	if config.GracePeriod == 0 {
		config.GracePeriod = 30 * time.Second
	}
	if config.WatermarkInterval == 0 {
		config.WatermarkInterval = 1 * time.Second
	}

	return &JoinOperator{
		config: config,
		leftState: &joinStateStore{
			events:  make(map[string][]*timedEvent),
			expiry:  config.WindowDuration + config.GracePeriod,
			maxSize: 100000,
		},
		rightState: &joinStateStore{
			events:  make(map[string][]*timedEvent),
			expiry:  config.WindowDuration + config.GracePeriod,
			maxSize: 100000,
		},
		outputChan: make(chan *JoinResult, 10000),
		metrics:    &JoinMetrics{},
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start starts the join operator.
func (j *JoinOperator) Start() {
	go j.evictionLoop()
	go j.watermarkLoop()
}

// Stop stops the join operator.
func (j *JoinOperator) Stop() {
	j.cancel()
}

// ProcessEvent processes an incoming event.
func (j *JoinOperator) ProcessEvent(ctx context.Context, event *Event) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	// Determine which stream this event belongs to
	isLeft := event.Type == j.config.LeftStream
	isRight := event.Type == j.config.RightStream

	if !isLeft && !isRight {
		return nil // Not for this join
	}

	// Get join key
	key := j.getJoinKey(event, isLeft)
	if key == "" {
		j.metrics.mu.Lock()
		j.metrics.DroppedEvents++
		j.metrics.mu.Unlock()
		return nil
	}

	// Get event timestamp
	eventTime := j.getEventTime(event)

	// Check if event is too late
	if !j.watermark.IsZero() && eventTime.Add(j.config.GracePeriod).Before(j.watermark) {
		j.metrics.mu.Lock()
		j.metrics.LateEvents++
		j.metrics.mu.Unlock()
		return nil
	}

	// Add event to state
	te := &timedEvent{
		event:     event,
		timestamp: eventTime,
		processed: time.Now(),
	}

	var matches []*timedEvent
	if isLeft {
		j.leftState.add(key, te)
		j.metrics.mu.Lock()
		j.metrics.LeftEvents++
		j.metrics.mu.Unlock()
		matches = j.rightState.get(key)
	} else {
		j.rightState.add(key, te)
		j.metrics.mu.Lock()
		j.metrics.RightEvents++
		j.metrics.mu.Unlock()
		matches = j.leftState.get(key)
	}

	// Process matches
	for _, match := range matches {
		// Check temporal alignment
		if !j.isInWindow(te.timestamp, match.timestamp) {
			continue
		}

		var result *JoinResult
		if isLeft {
			result = j.createJoinResult(key, event, match.event, eventTime)
		} else {
			result = j.createJoinResult(key, match.event, event, eventTime)
		}

		j.emitResult(result)

		j.metrics.mu.Lock()
		j.metrics.JoinMatches++
		j.metrics.mu.Unlock()
	}

	// Handle outer joins - emit unmatched events when window closes
	if (j.config.Type == JoinTypeLeft || j.config.Type == JoinTypeFull) && isLeft && len(matches) == 0 {
		// Will be emitted during window close
	}
	if (j.config.Type == JoinTypeRight || j.config.Type == JoinTypeFull) && isRight && len(matches) == 0 {
		// Will be emitted during window close
	}

	return nil
}

func (j *JoinOperator) getJoinKey(event *Event, isLeft bool) string {
	keyField := j.config.JoinKey
	if isLeft && j.config.LeftKey != "" {
		keyField = j.config.LeftKey
	} else if !isLeft && j.config.RightKey != "" {
		keyField = j.config.RightKey
	}

	if event.Data == nil {
		return event.EntityID // Fall back to entity ID
	}

	if val, ok := event.Data[keyField]; ok {
		return fmt.Sprintf("%v", val)
	}

	return event.EntityID
}

func (j *JoinOperator) getEventTime(event *Event) time.Time {
	if j.config.TimestampField != "" && event.Data != nil {
		if ts, ok := event.Data[j.config.TimestampField]; ok {
			switch v := ts.(type) {
			case time.Time:
				return v
			case string:
				if t, err := time.Parse(time.RFC3339, v); err == nil {
					return t
				}
			case int64:
				return time.Unix(0, v)
			case float64:
				return time.Unix(0, int64(v))
			}
		}
	}
	if !event.Timestamp.IsZero() {
		return event.Timestamp
	}
	return time.Now()
}

func (j *JoinOperator) isInWindow(t1, t2 time.Time) bool {
	diff := t1.Sub(t2)
	if diff < 0 {
		diff = -diff
	}
	return diff <= j.config.WindowDuration
}

func (j *JoinOperator) createJoinResult(key string, left, right *Event, eventTime time.Time) *JoinResult {
	output := make(map[string]interface{})

	// Include configured output fields
	for _, field := range j.config.OutputFields {
		var source *Event
		if field.Source == "left" {
			source = left
		} else {
			source = right
		}

		if source != nil && source.Data != nil {
			if val, ok := source.Data[field.Field]; ok {
				name := field.Field
				if field.Alias != "" {
					name = field.Alias
				}
				output[name] = val
			}
		}
	}

	// If no fields configured, include all
	if len(j.config.OutputFields) == 0 {
		if left != nil && left.Data != nil {
			for k, v := range left.Data {
				output["left_"+k] = v
			}
		}
		if right != nil && right.Data != nil {
			for k, v := range right.Data {
				output["right_"+k] = v
			}
		}
	}

	return &JoinResult{
		JoinName:    j.config.Name,
		JoinKey:     key,
		LeftEvent:   left,
		RightEvent:  right,
		OutputData:  output,
		EventTime:   eventTime,
		ProcessTime: time.Now(),
	}
}

func (j *JoinOperator) emitResult(result *JoinResult) {
	select {
	case j.outputChan <- result:
	default:
		// Buffer full
	}
}

// GetOutputChannel returns the output channel.
func (j *JoinOperator) GetOutputChannel() <-chan *JoinResult {
	return j.outputChan
}

// GetMetrics returns join metrics.
func (j *JoinOperator) GetMetrics() *JoinMetrics {
	j.metrics.mu.RLock()
	defer j.metrics.mu.RUnlock()

	j.mu.RLock()
	stateSize := int64(len(j.leftState.events) + len(j.rightState.events))
	j.mu.RUnlock()

	return &JoinMetrics{
		LeftEvents:     j.metrics.LeftEvents,
		RightEvents:    j.metrics.RightEvents,
		JoinMatches:    j.metrics.JoinMatches,
		LateEvents:     j.metrics.LateEvents,
		DroppedEvents:  j.metrics.DroppedEvents,
		StateSize:      stateSize,
		LastWatermark:  j.metrics.LastWatermark,
		AvgJoinLatency: j.metrics.AvgJoinLatency,
	}
}

func (j *JoinOperator) evictionLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-j.ctx.Done():
			return
		case <-ticker.C:
			j.evictExpired()
		}
	}
}

func (j *JoinOperator) evictExpired() {
	cutoff := time.Now().Add(-j.leftState.expiry)

	j.leftState.mu.Lock()
	for key, events := range j.leftState.events {
		var kept []*timedEvent
		for _, e := range events {
			if e.timestamp.After(cutoff) {
				kept = append(kept, e)
			} else if j.config.Type == JoinTypeLeft || j.config.Type == JoinTypeFull {
				// Emit unmatched left events for outer joins
				result := j.createJoinResult(key, e.event, nil, e.timestamp)
				j.emitResult(result)
			}
		}
		if len(kept) > 0 {
			j.leftState.events[key] = kept
		} else {
			delete(j.leftState.events, key)
		}
	}
	j.leftState.mu.Unlock()

	j.rightState.mu.Lock()
	for key, events := range j.rightState.events {
		var kept []*timedEvent
		for _, e := range events {
			if e.timestamp.After(cutoff) {
				kept = append(kept, e)
			} else if j.config.Type == JoinTypeRight || j.config.Type == JoinTypeFull {
				// Emit unmatched right events for outer joins
				result := j.createJoinResult(key, nil, e.event, e.timestamp)
				j.emitResult(result)
			}
		}
		if len(kept) > 0 {
			j.rightState.events[key] = kept
		} else {
			delete(j.rightState.events, key)
		}
	}
	j.rightState.mu.Unlock()
}

func (j *JoinOperator) watermarkLoop() {
	ticker := time.NewTicker(j.config.WatermarkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-j.ctx.Done():
			return
		case <-ticker.C:
			j.advanceWatermark()
		}
	}
}

func (j *JoinOperator) advanceWatermark() {
	j.mu.Lock()
	defer j.mu.Unlock()

	// Find minimum timestamp across all events
	var minTime time.Time

	j.leftState.mu.RLock()
	for _, events := range j.leftState.events {
		for _, e := range events {
			if minTime.IsZero() || e.timestamp.Before(minTime) {
				minTime = e.timestamp
			}
		}
	}
	j.leftState.mu.RUnlock()

	j.rightState.mu.RLock()
	for _, events := range j.rightState.events {
		for _, e := range events {
			if minTime.IsZero() || e.timestamp.Before(minTime) {
				minTime = e.timestamp
			}
		}
	}
	j.rightState.mu.RUnlock()

	if !minTime.IsZero() {
		newWatermark := minTime.Add(-j.config.GracePeriod)
		if newWatermark.After(j.watermark) {
			j.watermark = newWatermark
			j.metrics.mu.Lock()
			j.metrics.LastWatermark = newWatermark
			j.metrics.mu.Unlock()
		}
	}
}

// State store methods

func (s *joinStateStore) add(key string, event *timedEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	events := s.events[key]
	events = append(events, event)

	// Trim if too many events for this key
	if len(events) > s.maxSize/1000 {
		events = events[len(events)-s.maxSize/1000:]
	}

	s.events[key] = events
}

func (s *joinStateStore) get(key string) []*timedEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.events[key]
}

// JoinProcessor implements the Processor interface for joins.
type JoinProcessor struct {
	operator *JoinOperator
}

// NewJoinProcessor creates a new join processor for pipeline integration.
func NewJoinProcessor(config JoinConfig) *JoinProcessor {
	return &JoinProcessor{
		operator: NewJoinOperator(config),
	}
}

// Process processes an event through the join.
func (p *JoinProcessor) Process(ctx context.Context, event *Event) (*Event, error) {
	if err := p.operator.ProcessEvent(ctx, event); err != nil {
		return nil, err
	}
	// Pass through - join results come via output channel
	return event, nil
}

// Name returns the processor name.
func (p *JoinProcessor) Name() string {
	return "join:" + p.operator.config.Name
}

// GetOperator returns the underlying join operator.
func (p *JoinProcessor) GetOperator() *JoinOperator {
	return p.operator
}

// MultiJoinPipeline manages multiple joins across entities.
type MultiJoinPipeline struct {
	joins      map[string]*JoinOperator
	outputChan chan *JoinResult
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewMultiJoinPipeline creates a pipeline for multiple concurrent joins.
func NewMultiJoinPipeline() *MultiJoinPipeline {
	ctx, cancel := context.WithCancel(context.Background())
	return &MultiJoinPipeline{
		joins:      make(map[string]*JoinOperator),
		outputChan: make(chan *JoinResult, 10000),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// AddJoin adds a join to the pipeline.
func (p *MultiJoinPipeline) AddJoin(config JoinConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.joins[config.Name]; exists {
		return fmt.Errorf("join %s already exists", config.Name)
	}

	join := NewJoinOperator(config)
	join.Start()

	// Forward outputs
	go func() {
		for {
			select {
			case <-p.ctx.Done():
				return
			case result := <-join.GetOutputChannel():
				select {
				case p.outputChan <- result:
				default:
				}
			}
		}
	}()

	p.joins[config.Name] = join
	return nil
}

// RemoveJoin removes a join from the pipeline.
func (p *MultiJoinPipeline) RemoveJoin(name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	join, exists := p.joins[name]
	if !exists {
		return fmt.Errorf("join %s not found", name)
	}

	join.Stop()
	delete(p.joins, name)
	return nil
}

// ProcessEvent routes an event to relevant joins.
func (p *MultiJoinPipeline) ProcessEvent(ctx context.Context, event *Event) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, join := range p.joins {
		if event.Type == join.config.LeftStream || event.Type == join.config.RightStream {
			if err := join.ProcessEvent(ctx, event); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetOutputChannel returns the unified output channel.
func (p *MultiJoinPipeline) GetOutputChannel() <-chan *JoinResult {
	return p.outputChan
}

// Stop stops all joins.
func (p *MultiJoinPipeline) Stop() {
	p.cancel()
	p.mu.Lock()
	for _, join := range p.joins {
		join.Stop()
	}
	p.mu.Unlock()
}

// GetAllMetrics returns metrics for all joins.
func (p *MultiJoinPipeline) GetAllMetrics() map[string]*JoinMetrics {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]*JoinMetrics)
	for name, join := range p.joins {
		result[name] = join.GetMetrics()
	}
	return result
}

package federation

import (
	"fmt"
	"sync"
	"time"
)

// AggregationProtocol defines the secure aggregation protocol.
type AggregationProtocol string

// AggregationProtocol constants.
const (
	AggProtocolPlain     AggregationProtocol = "plain"
	AggProtocolMasked    AggregationProtocol = "masked"
	AggProtocolThreshold AggregationProtocol = "threshold"
)

// SecureAggConfig configures the secure aggregation.
type SecureAggConfig struct {
	Protocol  AggregationProtocol `json:"protocol"`
	Threshold int                 `json:"threshold"`
	Timeout   time.Duration       `json:"timeout"`
}

// DefaultSecureAggConfig returns default secure aggregation configuration.
func DefaultSecureAggConfig() SecureAggConfig {
	return SecureAggConfig{
		Protocol:  AggProtocolMasked,
		Threshold: 2,
		Timeout:   5 * time.Minute,
	}
}

// AggSessionStatus represents the status of an aggregation session.
type AggSessionStatus string

// AggSessionStatus constants.
const (
	AggStatusWaiting    AggSessionStatus = "waiting"
	AggStatusReady      AggSessionStatus = "ready"
	AggStatusAggregated AggSessionStatus = "aggregated"
	AggStatusFailed     AggSessionStatus = "failed"
)

// AggSession represents a secure aggregation session.
type AggSession struct {
	ID           string           `json:"id"`
	Participants []string         `json:"participants"`
	ReceivedFrom []string         `json:"received_from"`
	Status       AggSessionStatus `json:"status"`
	CreatedAt    time.Time        `json:"created_at"`
	AggregatedAt time.Time        `json:"aggregated_at,omitempty"`

	contributions map[string][]float64
	threshold     int
}

// AggResult holds the result of a secure aggregation.
type AggResult struct {
	SessionID        string    `json:"session_id"`
	Values           []float64 `json:"values"`
	ParticipantCount int       `json:"participant_count"`
	Timestamp        time.Time `json:"timestamp"`
}

// SecureAggregator manages secure aggregation sessions.
type SecureAggregator struct {
	mu       sync.RWMutex
	config   SecureAggConfig
	sessions map[string]*AggSession
}

// NewSecureAggregator creates a new secure aggregator.
func NewSecureAggregator(config SecureAggConfig) *SecureAggregator {
	return &SecureAggregator{
		config:   config,
		sessions: make(map[string]*AggSession),
	}
}

// CreateSession creates a new aggregation session.
func (sa *SecureAggregator) CreateSession(id string, participants []string) (*AggSession, error) {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	if id == "" {
		return nil, fmt.Errorf("session ID is required")
	}

	if _, exists := sa.sessions[id]; exists {
		return nil, fmt.Errorf("session %s already exists", id)
	}

	if len(participants) == 0 {
		return nil, fmt.Errorf("at least one participant is required")
	}

	threshold := sa.config.Threshold
	if threshold > len(participants) {
		threshold = len(participants)
	}

	session := &AggSession{
		ID:            id,
		Participants:  participants,
		ReceivedFrom:  make([]string, 0),
		Status:        AggStatusWaiting,
		CreatedAt:     time.Now(),
		contributions: make(map[string][]float64),
		threshold:     threshold,
	}

	sa.sessions[id] = session
	return session, nil
}

// SubmitContribution submits a participant's contribution to a session.
func (sa *SecureAggregator) SubmitContribution(sessionID, participantID string, values []float64) error {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	session, exists := sa.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	if session.Status == AggStatusAggregated {
		return fmt.Errorf("session %s already aggregated", sessionID)
	}

	if session.Status == AggStatusFailed {
		return fmt.Errorf("session %s has failed", sessionID)
	}

	// Verify participant is in the session
	found := false
	for _, p := range session.Participants {
		if p == participantID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("participant %s is not in session %s", participantID, sessionID)
	}

	// Check for duplicate
	if _, submitted := session.contributions[participantID]; submitted {
		return fmt.Errorf("participant %s already submitted", participantID)
	}

	session.contributions[participantID] = values
	session.ReceivedFrom = append(session.ReceivedFrom, participantID)

	if len(session.contributions) >= session.threshold {
		session.Status = AggStatusReady
	}

	return nil
}

// Aggregate computes the aggregate when the threshold is met.
func (sa *SecureAggregator) Aggregate(sessionID string) (*AggResult, error) {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	session, exists := sa.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	if session.Status == AggStatusAggregated {
		return nil, fmt.Errorf("session %s already aggregated", sessionID)
	}

	if len(session.contributions) < session.threshold {
		return nil, fmt.Errorf("session %s needs at least %d contributions, has %d",
			sessionID, session.threshold, len(session.contributions))
	}

	// Compute element-wise average
	var vectorLen int
	for _, values := range session.contributions {
		vectorLen = len(values)
		break
	}

	aggregated := make([]float64, vectorLen)
	count := 0
	for _, values := range session.contributions {
		for i := 0; i < vectorLen && i < len(values); i++ {
			aggregated[i] += values[i]
		}
		count++
	}

	// Average
	for i := range aggregated {
		aggregated[i] /= float64(count)
	}

	now := time.Now()
	session.Status = AggStatusAggregated
	session.AggregatedAt = now

	return &AggResult{
		SessionID:        sessionID,
		Values:           aggregated,
		ParticipantCount: count,
		Timestamp:        now,
	}, nil
}

// GetSession returns a session by ID.
func (sa *SecureAggregator) GetSession(id string) (*AggSession, error) {
	sa.mu.RLock()
	defer sa.mu.RUnlock()

	session, exists := sa.sessions[id]
	if !exists {
		return nil, fmt.Errorf("session %s not found", id)
	}

	return session, nil
}

// ListSessions returns all sessions.
func (sa *SecureAggregator) ListSessions() []*AggSession {
	sa.mu.RLock()
	defer sa.mu.RUnlock()

	sessions := make([]*AggSession, 0, len(sa.sessions))
	for _, s := range sa.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

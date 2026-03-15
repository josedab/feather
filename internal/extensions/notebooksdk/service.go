package notebooksdk

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"crypto/rand"
	"encoding/hex"
)

// Config holds configuration for the notebook SDK service.
type Config struct {
	MaxSessions           int
	SessionTimeoutMinutes int
	MaxResultRows         int
	EnableVisualization   bool
}

// DefaultConfig returns the default notebook SDK configuration.
func DefaultConfig() Config {
	return Config{
		MaxSessions:           100,
		SessionTimeoutMinutes: 60,
		MaxResultRows:         10000,
		EnableVisualization:   true,
	}
}

// SessionState represents the current state of a notebook session.
type SessionState string

const (
	SessionStateActive SessionState = "active"
	SessionStateClosed SessionState = "closed"
)

// SessionConfig holds parameters for creating a new session.
type SessionConfig struct {
	Notebook      string `json:"notebook"`
	User          string `json:"user"`
	ConnectionURL string `json:"connection_url"`
}

// Session represents an active notebook session.
type Session struct {
	ID           string
	Config       SessionConfig
	CreatedAt    time.Time
	LastActiveAt time.Time
	State        SessionState
}

// SessionInfo is a lightweight summary of a session.
type SessionInfo struct {
	ID        string
	User      string
	Notebook  string
	State     SessionState
	CreatedAt time.Time
}

// ExecutionResult holds the result of a magic command execution.
type ExecutionResult struct {
	Output         string
	Visualizations []Visualization
	Metadata       map[string]string
	DurationMs     int64
}

// VisualizationType identifies the kind of visualization.
type VisualizationType string

const (
	VisHistogram  VisualizationType = "histogram"
	VisDrift      VisualizationType = "drift"
	VisFreshness  VisualizationType = "freshness"
	VisLineage    VisualizationType = "lineage"
)

// Visualization carries rendered visualization data.
type Visualization struct {
	Type  VisualizationType
	Data  json.RawMessage
	Title string
}

// Stats holds aggregate statistics for the service.
type Stats struct {
	ActiveSessions  int
	TotalSessions   int
	TotalExecutions int64
}

// supportedCommands lists magic commands the service recognises.
var supportedCommands = map[string]bool{
	"connect":   true,
	"get":       true,
	"search":    true,
	"history":   true,
	"schema":    true,
	"visualize": true,
	"register":  true,
}

// Service is the notebook-native feature SDK service.
type Service struct {
	mu              sync.RWMutex
	cfg             Config
	sessions        map[string]*Session
	totalSessions   int
	totalExecutions int64
}

// NewService creates a new notebook SDK service with the given config.
func NewService(cfg Config) *Service {
	return &Service{
		cfg:      cfg,
		sessions: make(map[string]*Session),
	}
}

// CreateSession creates a new notebook session.
func (s *Service) CreateSession(cfg SessionConfig) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.sessions) >= s.cfg.MaxSessions {
		return nil, fmt.Errorf("notebooksdk: max sessions reached (%d)", s.cfg.MaxSessions)
	}

	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("notebooksdk: generating session id: %w", err)
	}

	now := time.Now()
	sess := &Session{
		ID:           id,
		Config:       cfg,
		CreatedAt:    now,
		LastActiveAt: now,
		State:        SessionStateActive,
	}
	s.sessions[id] = sess
	s.totalSessions++
	return sess, nil
}

// GetSession returns the session with the given ID.
func (s *Service) GetSession(id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, ok := s.sessions[id]
	if !ok {
		return nil, fmt.Errorf("notebooksdk: session %q not found", id)
	}
	return sess, nil
}

// CloseSession marks the session as closed and removes it.
func (s *Service) CloseSession(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[id]
	if !ok {
		return fmt.Errorf("notebooksdk: session %q not found", id)
	}
	sess.State = SessionStateClosed
	delete(s.sessions, id)
	return nil
}

// ListSessions returns info about all active sessions.
func (s *Service) ListSessions() []SessionInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	infos := make([]SessionInfo, 0, len(s.sessions))
	for _, sess := range s.sessions {
		infos = append(infos, SessionInfo{
			ID:        sess.ID,
			User:      sess.Config.User,
			Notebook:  sess.Config.Notebook,
			State:     sess.State,
			CreatedAt: sess.CreatedAt,
		})
	}
	return infos
}

// Execute runs a magic command within the given session.
// Commands follow the pattern: "%feather_<cmd> <args>".
func (s *Service) Execute(sessionID, command string) (*ExecutionResult, error) {
	s.mu.Lock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("notebooksdk: session %q not found", sessionID)
	}
	sess.LastActiveAt = time.Now()
	s.totalExecutions++
	s.mu.Unlock()

	start := time.Now()

	cmd, args, err := parseMagicCommand(command)
	if err != nil {
		return nil, fmt.Errorf("notebooksdk: parsing command: %w", err)
	}

	if !supportedCommands[cmd] {
		return nil, fmt.Errorf("notebooksdk: unsupported command %q", cmd)
	}

	result, err := s.dispatch(sess, cmd, args)
	if err != nil {
		return nil, err
	}

	result.DurationMs = time.Since(start).Milliseconds()
	return result, nil
}

// GenerateHistogram produces a histogram visualization for a feature.
func (s *Service) GenerateHistogram(sessionID, feature string) (*Visualization, error) {
	if !s.cfg.EnableVisualization {
		return nil, fmt.Errorf("notebooksdk: visualization disabled")
	}
	if _, err := s.GetSession(sessionID); err != nil {
		return nil, err
	}

	data, _ := json.Marshal(map[string]interface{}{
		"feature": feature,
		"bins":    20,
		"type":    "histogram",
	})
	return &Visualization{
		Type:  VisHistogram,
		Data:  data,
		Title: fmt.Sprintf("Distribution of %s", feature),
	}, nil
}

// GenerateDriftChart produces a drift chart visualization for a feature.
func (s *Service) GenerateDriftChart(sessionID, feature string) (*Visualization, error) {
	if !s.cfg.EnableVisualization {
		return nil, fmt.Errorf("notebooksdk: visualization disabled")
	}
	if _, err := s.GetSession(sessionID); err != nil {
		return nil, err
	}

	data, _ := json.Marshal(map[string]interface{}{
		"feature":   feature,
		"metric":    "psi",
		"threshold": 0.2,
	})
	return &Visualization{
		Type:  VisDrift,
		Data:  data,
		Title: fmt.Sprintf("Drift analysis for %s", feature),
	}, nil
}

// GenerateFreshnessIndicator produces a freshness visualization for the session.
func (s *Service) GenerateFreshnessIndicator(sessionID string) (*Visualization, error) {
	if !s.cfg.EnableVisualization {
		return nil, fmt.Errorf("notebooksdk: visualization disabled")
	}
	if _, err := s.GetSession(sessionID); err != nil {
		return nil, err
	}

	data, _ := json.Marshal(map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"status":    "fresh",
	})
	return &Visualization{
		Type:  VisFreshness,
		Data:  data,
		Title: "Feature Freshness",
	}, nil
}

// Stats returns aggregate service statistics.
func (s *Service) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return Stats{
		ActiveSessions:  len(s.sessions),
		TotalSessions:   s.totalSessions,
		TotalExecutions: s.totalExecutions,
	}
}

// parseMagicCommand extracts the command name and argument string from a magic
// command such as "%feather_get entity:123 feature1,feature2".
func parseMagicCommand(raw string) (cmd string, args string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", fmt.Errorf("empty command")
	}

	// Strip optional leading %feather_ prefix.
	raw = strings.TrimPrefix(raw, "%feather_")
	raw = strings.TrimPrefix(raw, "%")

	parts := strings.SplitN(raw, " ", 2)
	cmd = strings.ToLower(strings.TrimSpace(parts[0]))
	if cmd == "" {
		return "", "", fmt.Errorf("empty command name")
	}
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}
	return cmd, args, nil
}

// dispatch routes a parsed command to the appropriate handler.
func (s *Service) dispatch(sess *Session, cmd, args string) (*ExecutionResult, error) {
	switch cmd {
	case "connect":
		return s.execConnect(sess, args)
	case "get":
		return s.execGet(sess, args)
	case "search":
		return s.execSearch(sess, args)
	case "history":
		return s.execHistory(sess, args)
	case "schema":
		return s.execSchema(sess, args)
	case "visualize":
		return s.execVisualize(sess, args)
	case "register":
		return s.execRegister(sess, args)
	default:
		return nil, fmt.Errorf("notebooksdk: unhandled command %q", cmd)
	}
}

func (s *Service) execConnect(sess *Session, args string) (*ExecutionResult, error) {
	url := args
	if url == "" {
		url = sess.Config.ConnectionURL
	}
	if url == "" {
		return nil, fmt.Errorf("notebooksdk: connect requires a URL")
	}
	return &ExecutionResult{
		Output:   fmt.Sprintf("Connected to %s", url),
		Metadata: map[string]string{"url": url},
	}, nil
}

func (s *Service) execGet(_ *Session, args string) (*ExecutionResult, error) {
	if args == "" {
		return nil, fmt.Errorf("notebooksdk: get requires entity and features (e.g. entity:123 feature1,feature2)")
	}
	parts := strings.Fields(args)
	entity := parts[0]
	var features string
	if len(parts) > 1 {
		features = parts[1]
	}
	return &ExecutionResult{
		Output:   fmt.Sprintf("Features for %s: %s", entity, features),
		Metadata: map[string]string{"entity": entity, "features": features, "max_rows": fmt.Sprintf("%d", s.cfg.MaxResultRows)},
	}, nil
}

func (s *Service) execSearch(_ *Session, args string) (*ExecutionResult, error) {
	if args == "" {
		return nil, fmt.Errorf("notebooksdk: search requires a query")
	}
	return &ExecutionResult{
		Output:   fmt.Sprintf("Search results for: %s", args),
		Metadata: map[string]string{"query": args},
	}, nil
}

func (s *Service) execHistory(_ *Session, args string) (*ExecutionResult, error) {
	if args == "" {
		return nil, fmt.Errorf("notebooksdk: history requires entity and feature")
	}
	return &ExecutionResult{
		Output:   fmt.Sprintf("History for: %s", args),
		Metadata: map[string]string{"args": args},
	}, nil
}

func (s *Service) execSchema(_ *Session, args string) (*ExecutionResult, error) {
	target := "all"
	if args != "" {
		target = args
	}
	return &ExecutionResult{
		Output:   fmt.Sprintf("Schema for: %s", target),
		Metadata: map[string]string{"target": target},
	}, nil
}

func (s *Service) execVisualize(sess *Session, args string) (*ExecutionResult, error) {
	if !s.cfg.EnableVisualization {
		return nil, fmt.Errorf("notebooksdk: visualization disabled")
	}
	if args == "" {
		return nil, fmt.Errorf("notebooksdk: visualize requires a feature name")
	}

	vis, err := s.GenerateHistogram(sess.ID, args)
	if err != nil {
		return nil, err
	}
	return &ExecutionResult{
		Output:         fmt.Sprintf("Visualization for: %s", args),
		Visualizations: []Visualization{*vis},
		Metadata:       map[string]string{"feature": args},
	}, nil
}

func (s *Service) execRegister(_ *Session, args string) (*ExecutionResult, error) {
	if args == "" {
		return nil, fmt.Errorf("notebooksdk: register requires a feature definition")
	}
	return &ExecutionResult{
		Output:   fmt.Sprintf("Registered feature: %s", args),
		Metadata: map[string]string{"registered": args},
	}, nil
}

func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

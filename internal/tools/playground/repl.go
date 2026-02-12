package playground

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// REPLSession represents an interactive playground session.
type REPLSession struct {
	ID        string        `json:"id"`
	CreatedAt time.Time     `json:"created_at"`
	LastUsed  time.Time     `json:"last_used"`
	History   []REPLCommand `json:"history"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

// REPLCommand represents a single command in a REPL session.
type REPLCommand struct {
	Input     string      `json:"input"`
	Output    interface{} `json:"output,omitempty"`
	Error     string      `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
	ExecutedAt time.Time  `json:"executed_at"`
}

// REPLEngine manages interactive playground sessions.
type REPLEngine struct {
	mu       sync.RWMutex
	sessions map[string]*REPLSession
	provider FeatureProvider
	maxHistory int
}

// NewREPLEngine creates a new REPL engine.
func NewREPLEngine(provider FeatureProvider) *REPLEngine {
	return &REPLEngine{
		sessions:   make(map[string]*REPLSession),
		provider:   provider,
		maxHistory: 1000,
	}
}

// CreateSession creates a new REPL session.
func (e *REPLEngine) CreateSession() *REPLSession {
	e.mu.Lock()
	defer e.mu.Unlock()

	session := &REPLSession{
		ID:        uuid.New().String(),
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		Variables: make(map[string]interface{}),
	}
	e.sessions[session.ID] = session
	return session
}

// GetSession retrieves a session by ID.
func (e *REPLEngine) GetSession(id string) (*REPLSession, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	s, ok := e.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session %q not found", id)
	}
	return s, nil
}

// ListSessions returns all active sessions.
func (e *REPLEngine) ListSessions() []*REPLSession {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*REPLSession, 0, len(e.sessions))
	for _, s := range e.sessions {
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].LastUsed.After(result[j].LastUsed)
	})
	return result
}

// Execute runs a command in a session.
func (e *REPLEngine) Execute(sessionID, input string) *REPLCommand {
	e.mu.Lock()
	defer e.mu.Unlock()

	session, ok := e.sessions[sessionID]
	if !ok {
		return &REPLCommand{Input: input, Error: "session not found"}
	}

	start := time.Now()
	cmd := &REPLCommand{
		Input:      input,
		ExecutedAt: start,
	}

	output, err := e.executeCommand(session, input)
	cmd.Duration = time.Since(start)
	if err != nil {
		cmd.Error = err.Error()
	} else {
		cmd.Output = output
	}

	session.History = append(session.History, *cmd)
	if len(session.History) > e.maxHistory {
		session.History = session.History[len(session.History)-e.maxHistory:]
	}
	session.LastUsed = time.Now()

	return cmd
}

// DeleteSession removes a session.
func (e *REPLEngine) DeleteSession(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.sessions[id]; !ok {
		return fmt.Errorf("session %q not found", id)
	}
	delete(e.sessions, id)
	return nil
}

func (e *REPLEngine) executeCommand(session *REPLSession, input string) (interface{}, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty command")
	}

	parts := strings.Fields(input)
	command := strings.ToLower(parts[0])

	switch command {
	case "help":
		return e.cmdHelp(), nil
	case "get":
		return e.cmdGet(parts[1:])
	case "set":
		return e.cmdSet(session, parts[1:])
	case "list":
		return e.cmdList(parts[1:])
	case "describe":
		return e.cmdDescribe(parts[1:])
	case "history":
		return e.cmdHistory(session), nil
	case "vars":
		return session.Variables, nil
	case "clear":
		session.History = nil
		session.Variables = make(map[string]interface{})
		return "session cleared", nil
	default:
		return nil, fmt.Errorf("unknown command %q; type 'help' for available commands", command)
	}
}

func (e *REPLEngine) cmdHelp() interface{} {
	return map[string]string{
		"get <entity> [feature...]":   "Retrieve features for an entity",
		"set <var> <value>":           "Set a session variable",
		"list groups|queries|datasets": "List available resources",
		"describe <group>":            "Describe a feature group",
		"history":                     "Show command history",
		"vars":                        "Show session variables",
		"clear":                       "Clear session state",
		"help":                        "Show this help message",
	}
}

func (e *REPLEngine) cmdGet(args []string) (interface{}, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("usage: get <entity_key> [feature...]")
	}
	if e.provider == nil {
		return nil, fmt.Errorf("no feature provider configured")
	}

	entity := args[0]
	features := args[1:]
	if len(features) == 0 {
		// List available features and return them.
		allFeatures, err := e.provider.ListFeatures(nil)
		if err != nil {
			return nil, fmt.Errorf("listing features: %w", err)
		}
		return map[string]interface{}{
			"entity":            entity,
			"available_features": allFeatures,
			"hint":              "specify feature names: get <entity> <feature1> <feature2>",
		}, nil
	}

	result := make(map[string]interface{})
	for _, f := range features {
		val, ts, err := e.provider.GetFeature(nil, entity, f)
		if err != nil {
			result[f] = map[string]interface{}{"error": err.Error()}
		} else {
			result[f] = map[string]interface{}{"value": val, "timestamp": ts}
		}
	}
	return result, nil
}

func (e *REPLEngine) cmdSet(session *REPLSession, args []string) (interface{}, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("usage: set <variable> <value>")
	}
	session.Variables[args[0]] = strings.Join(args[1:], " ")
	return fmt.Sprintf("%s = %s", args[0], strings.Join(args[1:], " ")), nil
}

func (e *REPLEngine) cmdList(args []string) (interface{}, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("usage: list groups|queries|datasets")
	}
	switch strings.ToLower(args[0]) {
	case "groups", "features":
		if e.provider == nil {
			return nil, fmt.Errorf("no feature provider configured")
		}
		features, err := e.provider.ListFeatures(nil)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"features": features, "count": len(features)}, nil
	default:
		return nil, fmt.Errorf("unknown list target %q; try: groups, features", args[0])
	}
}

func (e *REPLEngine) cmdDescribe(args []string) (interface{}, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("usage: describe <feature_name>")
	}
	if e.provider == nil {
		return nil, fmt.Errorf("no feature provider configured")
	}
	values, err := e.provider.GetFeatureValues(nil, args[0], 10)
	if err != nil {
		return nil, fmt.Errorf("describing %q: %w", args[0], err)
	}
	return map[string]interface{}{
		"feature":      args[0],
		"sample_count": len(values),
		"samples":      values,
	}, nil
}

func (e *REPLEngine) cmdHistory(session *REPLSession) interface{} {
	if len(session.History) == 0 {
		return "no commands in history"
	}
	lines := make([]string, len(session.History))
	for i, cmd := range session.History {
		status := "ok"
		if cmd.Error != "" {
			status = "error"
		}
		lines[i] = fmt.Sprintf("[%d] %s (%s, %v)", i+1, cmd.Input, status, cmd.Duration)
	}
	return strings.Join(lines, "\n")
}

// TutorialStep represents a step in a guided tutorial.
type TutorialStep struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Command     string `json:"command"`
	Expected    string `json:"expected,omitempty"`
}

// Tutorial represents a guided playground tutorial.
type Tutorial struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Steps       []TutorialStep `json:"steps"`
}

// BuiltinTutorials returns the pre-built tutorials.
func BuiltinTutorials() []Tutorial {
	return []Tutorial{
		{
			Name:        "quickstart",
			Description: "Learn the basics of the Feather feature store",
			Steps: []TutorialStep{
				{Title: "List Feature Groups", Description: "See what feature groups are available", Command: "list groups"},
				{Title: "Get Features", Description: "Retrieve features for a user entity", Command: "get user:123 click_count purchase_total"},
				{Title: "Set Variables", Description: "Save values for later use", Command: "set entity user:456"},
				{Title: "View History", Description: "See your command history", Command: "history"},
			},
		},
		{
			Name:        "data-exploration",
			Description: "Explore and analyze feature data",
			Steps: []TutorialStep{
				{Title: "Describe a Group", Description: "See the schema of a feature group", Command: "describe user_features"},
				{Title: "Get Feature Values", Description: "Retrieve specific features", Command: "get user:100 click_count"},
			},
		},
	}
}

// ShareableState encodes a session's state for URL sharing.
type ShareableState struct {
	SessionID string                 `json:"session_id"`
	Commands  []string               `json:"commands"`
	Variables map[string]interface{} `json:"variables,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

// ExportState exports a session's state for sharing.
func (e *REPLEngine) ExportState(sessionID string) (*ShareableState, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	session, ok := e.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}

	commands := make([]string, len(session.History))
	for i, cmd := range session.History {
		commands[i] = cmd.Input
	}

	return &ShareableState{
		SessionID: session.ID,
		Commands:  commands,
		Variables: session.Variables,
		CreatedAt: time.Now(),
	}, nil
}

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	primaryColor   = lipgloss.Color("#7C3AED") // Purple
	secondaryColor = lipgloss.Color("#10B981") // Green
	accentColor    = lipgloss.Color("#F59E0B") // Orange
	subtleColor    = lipgloss.Color("#6B7280") // Gray
	errorColor     = lipgloss.Color("#EF4444") // Red

	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(secondaryColor).
			Padding(0, 1)

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Background(primaryColor).
			Foreground(lipgloss.Color("#FFFFFF"))

	statusStyle = lipgloss.NewStyle().
			Foreground(subtleColor).
			Padding(1, 0)

	helpStyle = lipgloss.NewStyle().
			Foreground(subtleColor).
			MarginTop(1)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtleColor).
			Padding(1, 2)

	successStyle = lipgloss.NewStyle().
			Foreground(secondaryColor)

	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor)

	labelStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true)
)

// View represents different screens
type View int

const (
	ViewDashboard View = iota
	ViewFeatureGroups
	ViewFeatures
	ViewQuery
	ViewVectors
	ViewHealth
)

// keyMap defines keybindings
type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Left    key.Binding
	Right   key.Binding
	Enter   key.Binding
	Back    key.Binding
	Query   key.Binding
	Vectors key.Binding
	Health  key.Binding
	Refresh key.Binding
	Help    key.Binding
	Quit    key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Back, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Enter, k.Back, k.Refresh},
		{k.Query, k.Vectors, k.Health},
		{k.Help, k.Quit},
	}
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "left"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "right"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc", "backspace"),
		key.WithHelp("esc", "back"),
	),
	Query: key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("q", "query features"),
	),
	Vectors: key.NewBinding(
		key.WithKeys("v"),
		key.WithHelp("v", "vectors"),
	),
	Health: key.NewBinding(
		key.WithKeys("H"),
		key.WithHelp("H", "health"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c", "Q"),
		key.WithHelp("ctrl+c", "quit"),
	),
}

// Model represents the application state
type Model struct {
	serverURL string
	view      View
	prevView  View

	// UI components
	groupsTable   table.Model
	featuresTable table.Model
	vectorsTable  table.Model
	queryInput    textinput.Model
	help          help.Model

	// Data
	groups        []FeatureGroup
	currentGroup  string
	features      []Feature
	vectorIndexes []VectorIndex
	healthStatus  HealthStatus
	queryResult   string

	// State
	width    int
	height   int
	loading  bool
	err      error
	showHelp bool
}

// Placeholder types (would be populated from API)
type FeatureGroup struct {
	Name        string
	EntityType  string
	TTL         int
	Description string
	Features    int
}

type Feature struct {
	Name     string
	DataType string
	Default  string
}

type VectorIndex struct {
	Name         string
	Dimension    int
	DistanceType string
	Size         int
}

type HealthStatus struct {
	Status     string
	Hot        string
	Warm       string
	Aggregator string
}

func initialModel(serverURL string) Model {
	// Initialize query input
	ti := textinput.New()
	ti.Placeholder = "entity:user:123 features:purchase_count,score"
	ti.CharLimit = 256
	ti.Width = 60

	// Initialize help
	h := help.New()

	// Create groups table
	groupsCols := []table.Column{
		{Title: "Name", Width: 20},
		{Title: "Entity Type", Width: 15},
		{Title: "Features", Width: 10},
		{Title: "TTL", Width: 10},
		{Title: "Description", Width: 30},
	}
	groupsTable := table.New(
		table.WithColumns(groupsCols),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	// Style the table
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(subtleColor).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(primaryColor).
		Bold(true)
	groupsTable.SetStyles(s)

	// Features table
	featuresCols := []table.Column{
		{Title: "Name", Width: 25},
		{Title: "Type", Width: 15},
		{Title: "Default", Width: 20},
	}
	featuresTable := table.New(
		table.WithColumns(featuresCols),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	featuresTable.SetStyles(s)

	// Vectors table
	vectorsCols := []table.Column{
		{Title: "Index Name", Width: 25},
		{Title: "Dimension", Width: 12},
		{Title: "Distance", Width: 15},
		{Title: "Vectors", Width: 12},
	}
	vectorsTable := table.New(
		table.WithColumns(vectorsCols),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	vectorsTable.SetStyles(s)

	// Sample data
	groups := []FeatureGroup{
		{Name: "user_features", EntityType: "user", TTL: 86400, Description: "User behavior features", Features: 5},
		{Name: "product_features", EntityType: "product", TTL: 3600, Description: "Product catalog features", Features: 8},
		{Name: "session_features", EntityType: "session", TTL: 1800, Description: "Session-based features", Features: 3},
	}

	vectorIndexes := []VectorIndex{
		{Name: "product_embeddings", Dimension: 384, DistanceType: "cosine", Size: 10000},
		{Name: "user_embeddings", Dimension: 256, DistanceType: "cosine", Size: 5000},
	}

	return Model{
		serverURL:     serverURL,
		view:          ViewDashboard,
		groupsTable:   groupsTable,
		featuresTable: featuresTable,
		vectorsTable:  vectorsTable,
		queryInput:    ti,
		help:          h,
		groups:        groups,
		vectorIndexes: vectorIndexes,
		healthStatus: HealthStatus{
			Status:     "healthy",
			Hot:        "OK (256 MB used)",
			Warm:       "OK (1.2 GB used)",
			Aggregator: "OK (15 windows)",
		},
		width:  80,
		height: 24,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width

	case tea.KeyMsg:
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}

		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, keys.Help):
			m.showHelp = !m.showHelp
			return m, nil

		case key.Matches(msg, keys.Back):
			if m.view != ViewDashboard {
				m.view = ViewDashboard
			}
			return m, nil

		case key.Matches(msg, keys.Query):
			m.prevView = m.view
			m.view = ViewQuery
			m.queryInput.Focus()
			return m, textinput.Blink

		case key.Matches(msg, keys.Vectors):
			m.view = ViewVectors
			m.updateVectorsTable()
			return m, nil

		case key.Matches(msg, keys.Health):
			m.view = ViewHealth
			return m, nil
		}

		// View-specific handling
		switch m.view {
		case ViewDashboard:
			switch {
			case key.Matches(msg, keys.Enter):
				m.view = ViewFeatureGroups
				m.updateGroupsTable()
			}

		case ViewFeatureGroups:
			m.groupsTable, cmd = m.groupsTable.Update(msg)
			if key.Matches(msg, keys.Enter) {
				if row := m.groupsTable.SelectedRow(); len(row) > 0 {
					m.currentGroup = row[0]
					m.view = ViewFeatures
					m.updateFeaturesTable()
				}
			}
			return m, cmd

		case ViewFeatures:
			m.featuresTable, cmd = m.featuresTable.Update(msg)
			return m, cmd

		case ViewVectors:
			m.vectorsTable, cmd = m.vectorsTable.Update(msg)
			return m, cmd

		case ViewQuery:
			if key.Matches(msg, keys.Enter) {
				m.executeQuery()
				return m, nil
			}
			m.queryInput, cmd = m.queryInput.Update(msg)
			return m, cmd
		}

	}

	return m, cmd
}

func (m *Model) updateGroupsTable() {
	rows := make([]table.Row, len(m.groups))
	for i, g := range m.groups {
		rows[i] = table.Row{
			g.Name,
			g.EntityType,
			fmt.Sprintf("%d", g.Features),
			fmt.Sprintf("%ds", g.TTL),
			g.Description,
		}
	}
	m.groupsTable.SetRows(rows)
}

func (m *Model) updateFeaturesTable() {
	// Sample features for the selected group
	features := []Feature{
		{Name: "purchase_count", DataType: "int", Default: "0"},
		{Name: "avg_order_value", DataType: "float", Default: "0.0"},
		{Name: "last_purchase_date", DataType: "timestamp", Default: "null"},
		{Name: "loyalty_tier", DataType: "string", Default: "bronze"},
		{Name: "embedding", DataType: "vector[128]", Default: "null"},
	}
	m.features = features

	rows := make([]table.Row, len(features))
	for i, f := range features {
		rows[i] = table.Row{f.Name, f.DataType, f.Default}
	}
	m.featuresTable.SetRows(rows)
}

func (m *Model) updateVectorsTable() {
	rows := make([]table.Row, len(m.vectorIndexes))
	for i, v := range m.vectorIndexes {
		rows[i] = table.Row{
			v.Name,
			fmt.Sprintf("%d", v.Dimension),
			v.DistanceType,
			fmt.Sprintf("%d", v.Size),
		}
	}
	m.vectorsTable.SetRows(rows)
}

func (m *Model) executeQuery() {
	input := m.queryInput.Value()
	// Parse simple query format: entity:user:123 features:f1,f2
	parts := strings.Fields(input)
	var entity, features string
	for _, p := range parts {
		if strings.HasPrefix(p, "entity:") {
			entity = strings.TrimPrefix(p, "entity:")
		} else if strings.HasPrefix(p, "features:") {
			features = strings.TrimPrefix(p, "features:")
		}
	}

	if entity != "" && features != "" {
		m.queryResult = fmt.Sprintf("Query Result for %s:\n\n", entity)
		for i, f := range strings.Split(features, ",") {
			m.queryResult += fmt.Sprintf("  %s: <value_%d>\n", f, i+1)
		}
	} else {
		m.queryResult = "Invalid query format. Use: entity:<key> features:<f1,f2>"
	}
}

func (m Model) View() string {
	if m.showHelp {
		return m.renderHelp()
	}

	var content string

	switch m.view {
	case ViewDashboard:
		content = m.renderDashboard()
	case ViewFeatureGroups:
		content = m.renderFeatureGroups()
	case ViewFeatures:
		content = m.renderFeatures()
	case ViewQuery:
		content = m.renderQuery()
	case ViewVectors:
		content = m.renderVectors()
	case ViewHealth:
		content = m.renderHealth()
	}

	return content
}

func (m Model) renderDashboard() string {
	var b strings.Builder

	// Title
	title := titleStyle.Render("🪶 Feather Feature Store")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Server info
	b.WriteString(labelStyle.Render("Server: "))
	b.WriteString(m.serverURL)
	b.WriteString("\n\n")

	// Stats boxes
	statsRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.renderStatBox("Feature Groups", fmt.Sprintf("%d", len(m.groups)), secondaryColor),
		"  ",
		m.renderStatBox("Vector Indexes", fmt.Sprintf("%d", len(m.vectorIndexes)), primaryColor),
		"  ",
		m.renderStatBox("Status", m.healthStatus.Status, accentColor),
	)
	b.WriteString(statsRow)
	b.WriteString("\n\n")

	// Navigation
	nav := boxStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			headerStyle.Render("Navigation"),
			"",
			"  [Enter] Browse Feature Groups",
			"  [v]     Vector Indexes",
			"  [q]     Query Features",
			"  [H]     Health Status",
			"  [?]     Help",
			"  [Q]     Quit",
		),
	)
	b.WriteString(nav)
	b.WriteString("\n")

	// Status
	b.WriteString(statusStyle.Render("Press Enter to browse feature groups"))

	return b.String()
}

func (m Model) renderStatBox(label, value string, color lipgloss.Color) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Padding(0, 2).
		Width(20).
		Align(lipgloss.Center)

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		lipgloss.NewStyle().Foreground(color).Bold(true).Render(value),
		lipgloss.NewStyle().Foreground(subtleColor).Render(label),
	)

	return style.Render(content)
}

func (m Model) renderFeatureGroups() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Feature Groups"))
	b.WriteString("\n")
	b.WriteString(m.groupsTable.View())
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓: navigate • Enter: view features • Esc: back"))

	return b.String()
}

func (m Model) renderFeatures() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(fmt.Sprintf("Features: %s", m.currentGroup)))
	b.WriteString("\n")
	b.WriteString(m.featuresTable.View())
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓: navigate • Esc: back to groups"))

	return b.String()
}

func (m Model) renderQuery() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Query Features"))
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("Query: "))
	b.WriteString(m.queryInput.View())
	b.WriteString("\n\n")

	if m.queryResult != "" {
		b.WriteString(boxStyle.Render(m.queryResult))
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("Enter: execute • Esc: back"))

	return b.String()
}

func (m Model) renderVectors() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Vector Indexes"))
	b.WriteString("\n")
	b.WriteString(m.vectorsTable.View())
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓: navigate • Esc: back"))

	return b.String()
}

func (m Model) renderHealth() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Health Status"))
	b.WriteString("\n\n")

	status := m.healthStatus.Status
	if status == "healthy" {
		status = successStyle.Render("● HEALTHY")
	} else {
		status = errorStyle.Render("● " + strings.ToUpper(status))
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		fmt.Sprintf("Overall: %s", status),
		"",
		fmt.Sprintf("Hot Tier:    %s", m.healthStatus.Hot),
		fmt.Sprintf("Warm Tier:   %s", m.healthStatus.Warm),
		fmt.Sprintf("Aggregator:  %s", m.healthStatus.Aggregator),
	)

	b.WriteString(boxStyle.Render(content))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("r: refresh • Esc: back"))

	return b.String()
}

func (m Model) renderHelp() string {
	return m.help.View(keys)
}

func main() {
	serverURL := "http://localhost:8080"
	if len(os.Args) > 1 {
		serverURL = os.Args[1]
	}

	p := tea.NewProgram(initialModel(serverURL), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

package tui

import (
	"time"

	"agent-router/internal/proxy"
	"agent-router/internal/upstream"

	"github.com/charmbracelet/bubbletea"
)

// Message types for upstream changes
type UpstreamAdded struct{ Upstream *upstream.Upstream }
type UpstreamUpdated struct{ Upstream *upstream.Upstream; OldName string }
type UpstreamDeleted struct{ Name string }
type UpstreamToggled struct{ Upstream *upstream.Upstream } // For enable/disable toggle

// Message types for reload
type ReloadRequest struct{}
type ReloadComplete struct{ Error error }

// Message types for model selection
type ModelSelected struct{ Model string }

// Callbacks holds all TUI callback functions
type Callbacks struct {
	OnUpstreamAdded         func(u *upstream.Upstream)
	OnUpstreamUpdated       func(u *upstream.Upstream, oldName string)
	OnUpstreamDeleted       func(name string)
	OnUpstreamToggled       func(u *upstream.Upstream)
	OnDefaultModelChanged   func(model string)
	OnUpstreamModelSelected func(u *upstream.Upstream)
	OnReload               func() error
	OnPrimarySelected      func(u *upstream.Upstream) // Stub -- wired in Task 2
	OnPrimaryCleared       func()                     // Stub -- wired in Task 2
}

// Model holds all TUI state (exported for cmd/agent-router/main.go)
type Model struct {
	serviceName  string
	version      string
	port         int
	startTime    time.Time
	upstreams    []*upstream.Upstream
	logs         []proxy.RequestLog
	requestCount int64
	successCount int64
	defaultModel string // Current default model

	// Window size for responsive layout
	width  int
	height int

	// Navigation state
	selectedIndex int

	// Form state
	formMode  string              // "", "add", "edit"
	formData  upstream.Upstream   // Form working copy
	formField int                 // Current field index (0-6)

	// Model selection mode
	modelSelectMode   bool
	modelSelectIndex  int     // For Task 2: independent index for model-select (0=Auto, 1..N=upstreams)
	primaryUpstream   *upstream.Upstream // For Task 2: current primary upstream (nil=auto)

	// Confirmation mode
	confirmMode bool
	confirmType string // "delete" or "shutdown"

	Callbacks Callbacks // Replaces individual On* function fields
}

// NewModel creates a new TUI model
func NewModel(serviceName, version string, port int, upstreams []*upstream.Upstream) Model {
	return Model{
		serviceName:    serviceName,
		version:        version,
		port:           port,
		startTime:      time.Now(),
		upstreams:      upstreams,
		logs:           make([]proxy.RequestLog, 0, 50),
		selectedIndex:  0,
		formMode:       "",
		confirmMode:    false,
		modelSelectMode: false,
		modelSelectIndex: 0,
		primaryUpstream: nil,
		width:          80,  // default
		height:         24,  // default
	}
}

// Init initializes the TUI model
func (m Model) Init() tea.Cmd {
	return nil
}

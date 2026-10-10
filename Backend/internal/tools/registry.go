package tools

import (
	"log"
	"sync"
)

// registryState holds the shared map and lock for a ToolRegistry. It is
// behind a pointer so ToolRegistry value copies all share the same state,
// and the mutex is never copied.
type registryState struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// ToolRegistry maps tool names to their implementations.
// It is safe for concurrent use: the MCP loader registers tools in the
// background while agent goroutines look them up during execution.
type ToolRegistry struct {
	state *registryState
}

// NewRegistry creates a ToolRegistry with all built-in tools registered.
func NewRegistry() ToolRegistry {
	r := NewEmptyRegistry()
	r.Register(&ReadFileTool{})
	r.Register(&WriteFileTool{})
	r.Register(&EditFileTool{})
	r.Register(&GrepTool{})
	r.Register(&GlobTool{})
	r.Register(&ListDirTool{})
	r.Register(&GitStatusTool{})
	r.Register(&GitDiffTool{})
	r.Register(&ShellTool{})
	return r
}

// NewEmptyRegistry creates a ToolRegistry with no tools registered.
// Useful for tests that want to register only specific fake tools.
func NewEmptyRegistry() ToolRegistry {
	return ToolRegistry{state: &registryState{tools: make(map[string]Tool)}}
}

// Register adds a tool to the registry. Later registrations with the same name overwrite earlier ones.
func (r ToolRegistry) Register(t Tool) {
	r.state.mu.Lock()
	defer r.state.mu.Unlock()
	r.state.tools[t.Name()] = t
}

// Get retrieves a tool by name. Returns nil if not found.
func (r ToolRegistry) Get(name string) Tool {
	r.state.mu.RLock()
	defer r.state.mu.RUnlock()
	return r.state.tools[name]
}

// Lookup retrieves a tool by name, reporting whether it was found.
func (r ToolRegistry) Lookup(name string) (Tool, bool) {
	r.state.mu.RLock()
	defer r.state.mu.RUnlock()
	t, ok := r.state.tools[name]
	return t, ok
}

// Names returns all registered tool names in sorted order.
func (r ToolRegistry) Names() []string {
	r.state.mu.RLock()
	names := make([]string, 0, len(r.state.tools))
	for name := range r.state.tools {
		names = append(names, name)
	}
	r.state.mu.RUnlock()
	// Simple insertion sort (small n)
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j] < names[j-1]; j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
	return names
}

// MCPManager manages MCP clients and their tools.
// Safe for concurrent use.
type MCPManager struct {
	mu      sync.Mutex
	clients map[string]MCPClientInterface
}

// MCPClientInterface is the interface for MCP client operations needed by the registry.
type MCPClientInterface interface {
	Close() error
}

// NewMCPManager creates a new MCP manager.
func NewMCPManager() *MCPManager {
	return &MCPManager{
		clients: make(map[string]MCPClientInterface),
	}
}

// RegisterClient registers an MCP client by connector ID.
func (m *MCPManager) RegisterClient(connectorID string, client MCPClientInterface) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[connectorID] = client
}

// UnregisterClient removes an MCP client by connector ID and closes it.
func (m *MCPManager) UnregisterClient(connectorID string) {
	m.mu.Lock()
	client, ok := m.clients[connectorID]
	if ok {
		delete(m.clients, connectorID)
	}
	m.mu.Unlock()
	if ok {
		if err := client.Close(); err != nil {
			log.Printf("close MCP client error for %s: %v", connectorID, err)
		}
	}
}

// GetClient returns an MCP client by connector ID.
func (m *MCPManager) GetClient(connectorID string) (MCPClientInterface, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	client, ok := m.clients[connectorID]
	return client, ok
}

// CloseAll closes all MCP clients.
func (m *MCPManager) CloseAll() {
	m.mu.Lock()
	clients := m.clients
	m.clients = make(map[string]MCPClientInterface)
	m.mu.Unlock()
	for id, client := range clients {
		if err := client.Close(); err != nil {
			log.Printf("close MCP client error for %s: %v", id, err)
		}
	}
}

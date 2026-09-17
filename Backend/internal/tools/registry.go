package tools

// ToolRegistry maps tool names to their implementations.
type ToolRegistry map[string]Tool

// NewRegistry creates a ToolRegistry with all built-in tools registered.
func NewRegistry() ToolRegistry {
	r := ToolRegistry{}
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

// Register adds a tool to the registry. Later registrations with the same name overwrite earlier ones.
func (r ToolRegistry) Register(t Tool) {
	r[t.Name()] = t
}

// Unregister removes a tool from the registry by name.
func (r ToolRegistry) Unregister(name string) {
	delete(r, name)
}

// Get retrieves a tool by name. Returns nil if not found.
func (r ToolRegistry) Get(name string) Tool {
	return r[name]
}

// Has checks if a tool is registered by name.
func (r ToolRegistry) Has(name string) bool {
	_, ok := r[name]
	return ok
}

// Names returns all registered tool names in sorted order.
func (r ToolRegistry) Names() []string {
	names := make([]string, 0, len(r))
	for name := range r {
		names = append(names, name)
	}
	// Simple insertion sort (small n)
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j] < names[j-1]; j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
	return names
}

// MCPManager manages MCP clients and their tools.
type MCPManager struct {
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
	m.clients[connectorID] = client
}

// UnregisterClient removes an MCP client by connector ID.
func (m *MCPManager) UnregisterClient(connectorID string) {
	if client, ok := m.clients[connectorID]; ok {
		_ = client.Close()
		delete(m.clients, connectorID)
	}
}

// GetClient returns an MCP client by connector ID.
func (m *MCPManager) GetClient(connectorID string) (MCPClientInterface, bool) {
	client, ok := m.clients[connectorID]
	return client, ok
}

// CloseAll closes all MCP clients.
func (m *MCPManager) CloseAll() {
	for id, client := range m.clients {
		_ = client.Close()
		delete(m.clients, id)
	}
}

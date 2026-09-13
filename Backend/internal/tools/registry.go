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

// Get retrieves a tool by name. Returns nil if not found.
func (r ToolRegistry) Get(name string) Tool {
	return r[name]
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

package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CodewithMubasher/NightCode/backend/internal/tools"
)

// MCPToolAdapter wraps an MCP tool definition as a tools.Tool interface.
type MCPToolAdapter struct {
	def       MCPToolDef
	client    *Client
	connectorID string
}

// NewMCPToolAdapter creates a tools.Tool from an MCP tool definition.
func NewMCPToolAdapter(def MCPToolDef, client *Client, connectorID string) *MCPToolAdapter {
	return &MCPToolAdapter{
		def:        def,
		client:     client,
		connectorID: connectorID,
	}
}

func (t *MCPToolAdapter) Name() string {
	return "mcp_" + t.connectorID + "_" + t.def.Name
}

func (t *MCPToolAdapter) Description() string {
	return t.def.Description
}

func (t *MCPToolAdapter) InputSchema() json.RawMessage {
	return t.def.InputSchema
}

func (t *MCPToolAdapter) Execute(ctx context.Context, input json.RawMessage, ws *tools.Workspace) (tools.ToolResult, error) {
	result, err := t.client.CallTool(ctx, t.def.Name, input)
	if err != nil {
		return tools.ToolResult{}, fmt.Errorf("mcp tool %s: %w", t.def.Name, err)
	}

	if result.IsError {
		// Extract error text from content.
		errMsg := ""
		for _, c := range result.Content {
			if c.Type == "text" && c.Text != "" {
				errMsg = c.Text
				break
			}
		}
		if errMsg == "" {
			errMsg = "mcp tool returned error"
		}
		return tools.ToolResult{}, &tools.ToolError{Code: "mcp_error", Message: errMsg}
	}

	// Concatenate text content.
	var output string
	for _, c := range result.Content {
		if c.Type == "text" {
			output += c.Text
		}
	}

	// Ensure output is valid JSON — wrap plain text in a JSON object.
	var rawOutput json.RawMessage
	if json.Valid([]byte(output)) {
		rawOutput = json.RawMessage(output)
	} else {
		wrapped, _ := json.Marshal(map[string]string{"result": output})
		rawOutput = wrapped
	}

	return tools.ToolResult{
		Output: rawOutput,
	}, nil
}

// IsMCPTool checks if a tool name belongs to an MCP connector.
func IsMCPTool(name string) bool {
	return len(name) > 4 && name[:4] == "mcp_"
}

// ExtractConnectorID extracts the connector ID from an MCP tool name.
// Format: mcp_{connectorID}_{toolName}
func ExtractConnectorID(toolName string) string {
	// Find the second underscore.
	count := 0
	for i, c := range toolName {
		if c == '_' {
			count++
			if count == 2 {
				return toolName[4:i]
			}
		}
	}
	return ""
}

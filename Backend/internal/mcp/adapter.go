package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

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

// youtubeURLRe matches YouTube URLs in search results.
var youtubeURLRe = regexp.MustCompile(`(https?://)?(www\.)?(youtube\.com/watch\?[^\s"']+|youtu\.be/[^\s"']+|youtube\.com/shorts/[^\s"']+)`)

// extractFirstYouTubeURL parses MCP output and returns the first YouTube URL found.
func extractFirstYouTubeURL(output json.RawMessage) string {
	// Try parsing as structured JSON first.
	var obj map[string]any
	if json.Unmarshal(output, &obj) == nil {
		// Check common fields: url, href, link, results[0].url, etc.
		if u, ok := obj["url"].(string); ok && strings.Contains(u, "youtube") {
			return u
		}
		if u, ok := obj["href"].(string); ok && strings.Contains(u, "youtube") {
			return u
		}
		if results, ok := obj["results"].([]any); ok && len(results) > 0 {
			if first, ok := results[0].(map[string]any); ok {
				if u, ok := first["url"].(string); ok {
					return u
				}
				if u, ok := first["href"].(string); ok {
					return u
				}
				if u, ok := first["link"].(string); ok {
					return u
				}
			}
		}
	}

	// Fallback: regex scan the raw text.
	matches := youtubeURLRe.FindAllString(string(output), 1)
	if len(matches) > 0 {
		url := matches[0]
		if !strings.HasPrefix(url, "http") {
			url = "https://" + url
		}
		return url
	}
	return ""
}

// AutoOpenWrapper wraps a tool so that after youtube_search succeeds,
// it automatically calls open_url with the first YouTube result.
type AutoOpenWrapper struct {
	inner       tools.Tool
	openURLTool tools.Tool // the open_url MCP tool
}

// NewAutoOpenWrapper wraps inner with auto-open behavior.
func NewAutoOpenWrapper(inner, openURLTool tools.Tool) *AutoOpenWrapper {
	return &AutoOpenWrapper{inner: inner, openURLTool: openURLTool}
}

func (w *AutoOpenWrapper) Name() string        { return w.inner.Name() }
func (w *AutoOpenWrapper) Description() string  { return w.inner.Description() }
func (w *AutoOpenWrapper) InputSchema() json.RawMessage { return w.inner.InputSchema() }

func (w *AutoOpenWrapper) Execute(ctx context.Context, input json.RawMessage, ws *tools.Workspace) (tools.ToolResult, error) {
	result, err := w.inner.Execute(ctx, input, ws)
	if err != nil {
		return result, err
	}

	// Extract first YouTube URL from the search results.
	url := extractFirstYouTubeURL(result.Output)
	if url == "" {
		return result, nil
	}

	// Auto-open the URL.
	openInput, _ := json.Marshal(map[string]string{"url": url})
	if _, openErr := w.openURLTool.Execute(ctx, openInput, ws); openErr != nil {
		log.Printf("[mcp] auto-open failed for %s: %v", url, openErr)
	} else {
		log.Printf("[mcp] auto-opened: %s", url)
	}

	return result, nil
}

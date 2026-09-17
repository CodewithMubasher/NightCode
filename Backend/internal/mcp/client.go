package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
	"sync/atomic"
)

// Client communicates with an MCP server over stdio (JSON-RPC 2.0).
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	mu     sync.Mutex
	reqID  atomic.Int64
	// pending maps request IDs to response channels.
	pending map[int]chan *JSONRPCResponse
dead    bool
}

// NewClient spawns the MCP server process and performs the initialize handshake.
// The ctx parameter controls only the handshake timeout — the process lifetime
// is independent and must be managed via Close().
func NewClient(ctx context.Context, command string, args []string, env []string) (*Client, error) {
	cmd := exec.Command(command, args...)
	if len(env) > 0 {
		cmd.Env = env
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start process: %w", err)
	}

	c := &Client{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewReader(stdoutPipe),
		pending: make(map[int]chan *JSONRPCResponse),
	}

	// Start response reader goroutine.
	go c.readLoop()

	// Start stderr reader goroutine.
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			log.Printf("[mcp] stderr: %s", scanner.Text())
		}
	}()

	// Wait for process exit in background to reap resources.
	go func() {
		if err := c.cmd.Wait(); err != nil {
			log.Printf("[mcp] process exited: %v", err)
		} else {
			log.Printf("[mcp] process exited cleanly")
		}
		c.mu.Lock()
		if !c.dead {
			c.dead = true
			for id, ch := range c.pending {
				ch <- &JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      id,
					Error:   &JSONRPCError{Code: -1, Message: "process exited"},
				}
				delete(c.pending, id)
			}
		}
		c.mu.Unlock()
	}()

	// Initialize handshake.
	if err := c.initialize(ctx); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("initialize: %w", err)
	}

	return c, nil
}

// initialize sends the MCP initialize request and waits for the response.
func (c *Client) initialize(ctx context.Context) error {
	params := InitializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities:    ClientCapabilities{},
		ClientInfo:      ClientInfo{Name: "nightcode", Version: "1.0.0"},
	}
	var result InitializeResult
	if err := c.sendRequest(ctx, "initialize", params, &result); err != nil {
		return err
	}
	// Send initialized notification (no response expected).
	return c.sendNotification("notifications/initialized")
}

// ListTools calls tools/list and returns the available tool definitions.
func (c *Client) ListTools(ctx context.Context) ([]MCPToolDef, error) {
	var result ListToolsResult
	if err := c.sendRequest(ctx, "tools/list", nil, &result); err != nil {
		return nil, err
	}
	return result.Tools, nil
}

// CallTool calls tools/call with the given tool name and arguments.
func (c *Client) CallTool(ctx context.Context, name string, args json.RawMessage) (*CallToolResult, error) {
	params := CallToolParams{Name: name, Arguments: args}
	var result CallToolResult
	if err := c.sendRequest(ctx, "tools/call", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Close shuts down the MCP server process.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.dead {
		c.mu.Unlock()
		return nil
	}
	c.dead = true
	c.mu.Unlock()

	_ = c.stdin.Close()
	return c.cmd.Process.Kill()
}

// --- internal JSON-RPC transport ---

func (c *Client) sendRequest(ctx context.Context, method string, params interface{}, result interface{}) error {
	id := int(c.reqID.Add(1))

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	data = append(data, '\n')

	// Register pending response channel.
	ch := make(chan *JSONRPCResponse, 1)
	c.mu.Lock()
	if c.dead {
		c.mu.Unlock()
		return fmt.Errorf("client closed")
	}
	c.pending[id] = ch
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	c.mu.Lock()
	_, err = c.stdin.Write(data)
	c.mu.Unlock()
	if err != nil {
		return fmt.Errorf("write request: %w", err)
	}

	// Wait for response or context cancellation.
	select {
	case resp := <-ch:
		if resp.Error != nil {
			return resp.Error
		}
		if result != nil && len(resp.Result) > 0 {
			if err := json.Unmarshal(resp.Result, result); err != nil {
				return fmt.Errorf("unmarshal result: %w", err)
			}
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) sendNotification(method string) error {
	notif := struct {
		JSONRPC string      `json:"jsonrpc"`
		Method  string      `json:"method"`
		Params  interface{} `json:"params,omitempty"`
	}{
		JSONRPC: "2.0",
		Method:  method,
	}
	data, err := json.Marshal(notif)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err = c.stdin.Write(data)
	return err
}

func (c *Client) readLoop() {
	for {
		line, err := c.stdout.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("[mcp] read error: %v", err)
			}
			log.Printf("[mcp] stdout closed (EOF), marking client dead")
			c.mu.Lock()
			c.dead = true
			// Unblock any pending requests.
			for id, ch := range c.pending {
				ch <- &JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      id,
					Error:   &JSONRPCError{Code: -1, Message: "client closed"},
				}
				delete(c.pending, id)
			}
			c.mu.Unlock()
			return
		}

		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		var resp JSONRPCResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			log.Printf("[mcp] unmarshal error: %v (line: %s)", err, string(line))
			continue
		}

		log.Printf("[mcp] received response id=%d (error=%v)", resp.ID, resp.Error != nil)

		c.mu.Lock()
		ch, ok := c.pending[resp.ID]
		c.mu.Unlock()
		if ok {
			ch <- &resp
		} else {
			log.Printf("[mcp] unexpected response id=%d (no pending request)", resp.ID)
		}
	}
}

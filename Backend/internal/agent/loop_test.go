package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/CodewithMubasher/NightCode/backend/internal/provider"
	"github.com/CodewithMubasher/NightCode/backend/internal/tools"
	"github.com/CodewithMubasher/NightCode/backend/internal/types"
)

// fakeProvider returns a scripted sequence of ChatEvents.
type fakeProvider struct {
	// responses is a list of ChatEvent lists; each StreamChat call pops one.
	responses [][]provider.ChatEvent
	callCount int
}

func (f *fakeProvider) StreamChat(ctx context.Context, req provider.ChatRequest) (<-chan provider.ChatEvent, error) {
	idx := f.callCount
	f.callCount++
	ch := make(chan provider.ChatEvent, 16)
	go func() {
		defer close(ch)
		var events []provider.ChatEvent
		if idx < len(f.responses) {
			events = f.responses[idx]
		}
		for _, e := range events {
			ch <- e
		}
	}()
	return ch, nil
}

func collect(events <-chan types.RuntimeEvent) []types.RuntimeEvent {
	var out []types.RuntimeEvent
	for e := range events {
		out = append(out, e)
	}
	return out
}

// fakeTool always fails, to drive consecutive failures.
type fakeFailTool struct{}

func (fakeFailTool) Name() string                     { return "fail_tool" }
func (fakeFailTool) Description() string              { return "always fails" }
func (fakeFailTool) InputSchema() json.RawMessage     { return json.RawMessage(`{"type":"object","properties":{}}`) }
func (fakeFailTool) Execute(ctx context.Context, input json.RawMessage, ws *tools.Workspace) (tools.ToolResult, error) {
	return tools.ToolResult{}, &tools.ToolError{Code: "boom", Message: "deliberate failure"}
}

func failRegistry() tools.ToolRegistry {
	r := tools.ToolRegistry{}
	r.Register(fakeFailTool{})
	return r
}

// TestLoop_ConsecutiveToolFailureCap verifies 4 consecutive tool failures aborts
// with turn.error / too_many_tool_failures (not looping to the 20-iteration cap).
func TestLoop_ConsecutiveToolFailureCap(t *testing.T) {
	ws, _ := tools.NewWorkspace(t.TempDir())

	// Three iterations, each returning a tool call to fail_tool.
	// 4th failure should trigger the cap, but we only script 4 iterations worth.
	call := provider.ToolCall{ID: "1", Name: "fail_tool", Arguments: "{}"}

	var responses [][]provider.ChatEvent
	for i := 0; i < maxIterations; i++ {
		responses = append(responses, []provider.ChatEvent{
			{Type: provider.ChatEventToolCalls, ToolCalls: []provider.ToolCall{call}},
		})
	}

	p := &fakeProvider{responses: responses}
	loop := NewLoop(p, failRegistry(), nil)

	events := make(chan types.RuntimeEvent, 64)
	go loop.Run(context.Background(), ws, "c", "do it", nil, events)
	evts := collect(events)

	// Count failures and confirm termination type
	failures := 0
	var termErr *types.RuntimeEvent
	for i := range evts {
		if evts[i].Type == "tool.failed" {
			failures++
		}
		if evts[i].Type == "turn.error" {
			termErr = &evts[i]
		}
	}

	if failures != maxConsecutiveFailures {
		t.Errorf("expected %d failures before abort, got %d", maxConsecutiveFailures, failures)
	}
	if termErr == nil {
		t.Fatal("expected a turn.error event")
	}
	if termErr.Code != "too_many_tool_failures" {
		t.Errorf("expected code too_many_tool_failures, got %q", termErr.Code)
	}
	if p.callCount > maxConsecutiveFailures {
		t.Errorf("loop kept iterating after failure cap: %d model calls", p.callCount)
	}
}

// noToolOkTool always succeeds.
type fakeOkTool struct{}

func (fakeOkTool) Name() string                     { return "ok_tool" }
func (fakeOkTool) Description() string              { return "always ok" }
func (fakeOkTool) InputSchema() json.RawMessage     { return json.RawMessage(`{"type":"object","properties":{}}`) }
func (fakeOkTool) Execute(ctx context.Context, input json.RawMessage, ws *tools.Workspace) (tools.ToolResult, error) {
	return tools.ToolResult{Output: json.RawMessage(`{"ok":true}`)}, nil
}

func okRegistry() tools.ToolRegistry {
	r := tools.ToolRegistry{}
	r.Register(fakeOkTool{})
	return r
}

// TestLoop_MaxIterationCap verifies the loop stops at 20 iterations with
// max_iterations_exceeded when the model keeps requesting tools.
func TestLoop_MaxIterationCap(t *testing.T) {
	ws, _ := tools.NewWorkspace(t.TempDir())
	call := provider.ToolCall{ID: "1", Name: "ok_tool", Arguments: "{}"}

	var responses [][]provider.ChatEvent
	for i := 0; i < maxIterations; i++ {
		responses = append(responses, []provider.ChatEvent{
			{Type: provider.ChatEventToolCalls, ToolCalls: []provider.ToolCall{call}},
		})
	}

	p := &fakeProvider{responses: responses}
	loop := NewLoop(p, okRegistry(), nil)

	events := make(chan types.RuntimeEvent, 128)
	go loop.Run(context.Background(), ws, "c", "loop forever", nil, events)
	evts := collect(events)

	var termErr *types.RuntimeEvent
	for i := range evts {
		if evts[i].Type == "turn.error" {
			termErr = &evts[i]
		}
	}
	if termErr == nil {
		t.Fatal("expected a turn.error event")
	}
	if termErr.Code != "max_iterations_exceeded" {
		t.Errorf("expected code max_iterations_exceeded, got %q", termErr.Code)
	}
	if p.callCount != maxIterations {
		t.Errorf("expected exactly %d model calls, got %d", maxIterations, p.callCount)
	}
}

// TestLoop_RunParallelToolCalls verifies multiple tool calls in one turn execute
// and share a single segment ID (parallel group).
func TestLoop_ParallelToolCalls(t *testing.T) {
	ws, _ := tools.NewWorkspace(t.TempDir())

	responses := [][]provider.ChatEvent{
		{
			{Type: provider.ChatEventToolCalls, ToolCalls: []provider.ToolCall{
				{ID: "a", Name: "ok_tool", Arguments: "{}"},
				{ID: "b", Name: "ok_tool", Arguments: "{}"},
				{ID: "c", Name: "ok_tool", Arguments: "{}"},
			}},
		},
		{
			{Type: provider.ChatEventDelta, Text: "done"},
		},
	}

	p := &fakeProvider{responses: responses}
	loop := NewLoop(p, okRegistry(), nil)

	events := make(chan types.RuntimeEvent, 128)
	go loop.Run(context.Background(), ws, "c", "parallel", nil, events)
	evts := collect(events)

	// All three tool.started should share the same segmentId
	segIDs := map[string]int{}
	started := 0
	for i := range evts {
		if evts[i].Type == "tool.started" {
			started++
			segIDs[evts[i].SegmentID]++
		}
	}
	if started != 3 {
		t.Errorf("expected 3 tool.started, got %d", started)
	}
	if len(segIDs) != 1 {
		t.Errorf("expected all parallel tools to share one segment ID, got %d", len(segIDs))
	}
}
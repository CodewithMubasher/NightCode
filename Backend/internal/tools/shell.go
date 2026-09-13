package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

const (
	shellDefaultTimeoutSec = 30
	shellMaxTimeoutSec     = 120
	shellMaxOutputBytes    = 100 * 1024 // 100KB, matches read_file's cap
)

type ShellInput struct {
	Command string `json:"command"`
	// TimeoutSec optionally overrides the default command timeout, capped at
	// shellMaxTimeoutSec so a single tool call can't hang the turn forever.
	TimeoutSec int `json:"timeout_sec,omitempty"`
}

type ShellOutput struct {
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exitCode"`
	TimedOut   bool   `json:"timedOut"`
	Truncated  bool   `json:"truncated"`
	DurationMs int64  `json:"durationMs"`
}

// ShellTool runs a shell command with the workspace root as its working
// directory. This is deliberately more powerful (and more dangerous) than
// the file tools — it is NOT sandboxed against reading/writing outside the
// workspace the way ResolvePath enforces for file tools, since a shell
// command can `cd`, follow symlinks, or invoke arbitrary binaries. The
// guardrails here are: bounded execution time, bounded output size, and
// running as the same OS user/permissions as the backend process (no
// privilege escalation). Treat this the same as giving the agent a terminal
// in the workspace directory, because that's what it is.
type ShellTool struct{}

func (t *ShellTool) Name() string { return "shell" }
func (t *ShellTool) Description() string {
	if runtime.GOOS == "windows" {
		return "Run a command in the workspace root directory using Windows PowerShell. Write commands in PowerShell syntax (e.g. Get-ChildItem, Remove-Item, $env:VAR, ; as a statement separator) — NOT bash/sh syntax (avoid things like ls, rm -rf, export VAR=, or && between commands; use PowerShell's own equivalents instead). Use for builds, tests, package installs, git operations not covered by other tools, and anything else that needs a real shell. Output is captured (stdout/stderr) and truncated if very large. Long-running or interactive commands will time out."
	}
	return "Run a shell command in the workspace root directory. Use for builds, tests, package installs, git operations not covered by other tools, and anything else that needs a real shell. Output is captured (stdout/stderr) and truncated if very large. Long-running or interactive commands will time out."
}
func (t *ShellTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "description": "The shell command to run"},
			"timeout_sec": {"type": "integer", "description": "Optional timeout in seconds (default 30, max 120)"}
		},
		"required": ["command"]
	}`)
}

func (t *ShellTool) Execute(ctx context.Context, input json.RawMessage, ws *Workspace) (ToolResult, error) {
	var in ShellInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ToolResult{}, &ToolError{Code: "invalid_input", Message: fmt.Sprintf("invalid JSON: %v", err)}
	}
	if in.Command == "" {
		return ToolResult{}, &ToolError{Code: "invalid_input", Message: "command is required"}
	}

	timeoutSec := in.TimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = shellDefaultTimeoutSec
	}
	if timeoutSec > shellMaxTimeoutSec {
		timeoutSec = shellMaxTimeoutSec
	}

	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Use real PowerShell, not cmd.exe — the tool description tells the
		// model to write PowerShell syntax, so execution must match or every
		// command will fail (different quoting, different builtins,
		// different statement separators).
		cmd = exec.CommandContext(runCtx, "powershell", "-NoProfile", "-NonInteractive", "-Command", in.Command)
	} else {
		cmd = exec.CommandContext(runCtx, "sh", "-c", in.Command)
	}
	cmd.Dir = ws.Root

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	started := time.Now()
	runErr := cmd.Run()
	duration := time.Since(started)

	timedOut := runCtx.Err() == context.DeadlineExceeded

	exitCode := 0
	if exitErr, ok := runErr.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if runErr != nil && !timedOut {
		// Command couldn't even start (e.g. shell not found) — treat as a
		// tool error rather than a normal non-zero exit, since there's no
		// useful stdout/stderr to show.
		return ToolResult{}, &ToolError{Code: "exec_error", Message: fmt.Sprintf("failed to run command: %v", runErr)}
	}

	outStr, outTrunc := truncateBytes(stdout.String(), shellMaxOutputBytes)
	errStr, errTrunc := truncateBytes(stderr.String(), shellMaxOutputBytes)

	out := ShellOutput{
		Stdout:     outStr,
		Stderr:     errStr,
		ExitCode:   exitCode,
		TimedOut:   timedOut,
		Truncated:  outTrunc || errTrunc,
		DurationMs: duration.Milliseconds(),
	}
	resultBytes, _ := json.Marshal(out)

	return ToolResult{Output: resultBytes}, nil
}

func truncateBytes(s string, max int) (string, bool) {
	if len(s) <= max {
		return s, false
	}
	return s[:max] + "\n... (truncated)", true
}

package dsml

import (
	"encoding/json"
	"testing"
)

func TestTextOnly(t *testing.T) {
	tr := New()
	resp, err := tr.Translate("Happy to help you get started with Python!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Happy to help you get started with Python!" {
		t.Errorf("content = %q", resp.Content)
	}
	if len(resp.ToolCalls) != 0 {
		t.Errorf("expected 0 tool calls, got %d", len(resp.ToolCalls))
	}
}

func TestOneToolCall(t *testing.T) {
	tr := New()
	input := `<｜｜DSML｜｜ calls>
<｜｜DSML｜｜ invoke name="list_dir">
<｜｜DSML｜｜ parameter name="path" string="true">.</｜｜DSML｜｜ parameter>
</｜｜DSML｜｜ invoke>
</｜｜DSML｜｜ calls>`

	resp, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "" {
		t.Errorf("expected empty content, got %q", resp.Content)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.Name != "list_dir" {
		t.Errorf("name = %q, want list_dir", tc.Name)
	}
	if tc.ID == "" {
		t.Error("ID should not be empty")
	}
	if tc.Arguments["path"] != "." {
		t.Errorf("arguments[path] = %q, want \".\"", tc.Arguments["path"])
	}
}

func TestTextAndOneToolCall(t *testing.T) {
	tr := New()
	input := `I'll create ` + "`main.py`" + ` with a Hello World program.

<｜｜DSML｜｜ calls>
<｜｜DSML｜｜ invoke name="create_file">
<｜｜DSML｜｜ parameter name="path" string="true">main.py</｜｜DSML｜｜ parameter>
<｜｜DSML｜｜ parameter name="content" string="true">print("Hello, World!")</｜｜DSML｜｜ parameter>
</｜｜DSML｜｜ invoke>
</｜｜DSML｜｜ calls>`

	resp, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "I'll create `main.py` with a Hello World program." {
		t.Errorf("content = %q", resp.Content)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.Name != "create_file" {
		t.Errorf("name = %q, want create_file", tc.Name)
	}
	if tc.Arguments["path"] != "main.py" {
		t.Errorf("arguments[path] = %q", tc.Arguments["path"])
	}
	if tc.Arguments["content"] != `print("Hello, World!")` {
		t.Errorf("arguments[content] = %q", tc.Arguments["content"])
	}
}

func TestTextAndMultipleToolCalls(t *testing.T) {
	tr := New()
	input := `I'll inspect the project structure and check the Git status.

<｜｜DSML｜｜ calls>
<｜｜DSML｜｜ invoke name="list_dir">
<｜｜DSML｜｜ parameter name="path" string="true">.</｜｜DSML｜｜ parameter>
</｜｜DSML｜｜ invoke>
<｜｜DSML｜｜ invoke name="git_status">

</｜｜DSML｜｜ invoke>
</｜｜DSML｜｜ calls>`

	resp, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "I'll inspect the project structure and check the Git status." {
		t.Errorf("content = %q", resp.Content)
	}
	if len(resp.ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "list_dir" {
		t.Errorf("tool 0 name = %q", resp.ToolCalls[0].Name)
	}
	if resp.ToolCalls[1].Name != "git_status" {
		t.Errorf("tool 1 name = %q", resp.ToolCalls[1].Name)
	}
}

func TestMultipleToolCallsNoText(t *testing.T) {
	tr := New()
	input := `<｜｜DSML｜｜ calls>
<｜｜DSML｜｜ invoke name="list_dir">
<｜｜DSML｜｜ parameter name="path" string="true">.</｜｜DSML｜｜ parameter>
</｜｜DSML｜｜ invoke>
<｜｜DSML｜｜ invoke name="git_status">

</｜｜DSML｜｜ invoke>
<｜｜DSML｜｜ invoke name="read_file">
<｜｜DSML｜｜ parameter name="path" string="true">README.md</｜｜DSML｜｜ parameter>
</｜｜DSML｜｜ invoke>
</｜｜DSML｜｜ calls>`

	resp, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "" {
		t.Errorf("expected empty content, got %q", resp.Content)
	}
	if len(resp.ToolCalls) != 3 {
		t.Fatalf("expected 3 tool calls, got %d", len(resp.ToolCalls))
	}
	names := []string{resp.ToolCalls[0].Name, resp.ToolCalls[1].Name, resp.ToolCalls[2].Name}
	expected := []string{"list_dir", "git_status", "read_file"}
	for i, e := range expected {
		if names[i] != e {
			t.Errorf("tool %d name = %q, want %q", i, names[i], e)
		}
	}
}

func TestToolWithZeroParameters(t *testing.T) {
	tr := New()
	input := `<｜｜DSML｜｜ calls>
<｜｜DSML｜｜ invoke name="git_status">

</｜｜DSML｜｜ invoke>
</｜｜DSML｜｜ calls>`

	resp, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.Name != "git_status" {
		t.Errorf("name = %q", tc.Name)
	}
	if len(tc.Arguments) != 0 {
		t.Errorf("expected empty arguments, got %v", tc.Arguments)
	}
}

func TestToolWithOneParameter(t *testing.T) {
	tr := New()
	input := `<｜｜DSML｜｜ calls>
<｜｜DSML｜｜ invoke name="read_file">
<｜｜DSML｜｜ parameter name="path" string="true">src/main.go</｜｜DSML｜｜ parameter>
</｜｜DSML｜｜ invoke>
</｜｜DSML｜｜ calls>`

	resp, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tc := resp.ToolCalls[0]
	if tc.Arguments["path"] != "src/main.go" {
		t.Errorf("arguments[path] = %q", tc.Arguments["path"])
	}
}

func TestToolWithMultipleParameters(t *testing.T) {
	tr := New()
	input := `<｜｜DSML｜｜ calls>
<｜｜DSML｜｜ invoke name="create_file">
<｜｜DSML｜｜ parameter name="path" string="true">src/utils.js</｜｜DSML｜｜ parameter>
<｜｜DSML｜｜ parameter name="content" string="true">export function add(a, b) { return a + b; }</｜｜DSML｜｜ parameter>
</｜｜DSML｜｜ invoke>
</｜｜DSML｜｜ calls>`

	resp, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tc := resp.ToolCalls[0]
	if tc.Arguments["path"] != "src/utils.js" {
		t.Errorf("arguments[path] = %q", tc.Arguments["path"])
	}
	if tc.Arguments["content"] != "export function add(a, b) { return a + b; }" {
		t.Errorf("arguments[content] = %q", tc.Arguments["content"])
	}
}

func TestMultilineFileContent(t *testing.T) {
	tr := New()
	input := `<｜｜DSML｜｜ calls>
<｜｜DSML｜｜ invoke name="create_file">
<｜｜DSML｜｜ parameter name="path" string="true">main.go</｜｜DSML｜｜ parameter>
<｜｜DSML｜｜ parameter name="content" string="true">package main

import "fmt"

func main() {
    fmt.Println("Hello")
}</｜｜DSML｜｜ parameter>
</｜｜DSML｜｜ invoke>
</｜｜DSML｜｜ calls>`

	resp, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tc := resp.ToolCalls[0]
	expected := `package main

import "fmt"

func main() {
    fmt.Println("Hello")
}`
	if tc.Arguments["content"] != expected {
		t.Errorf("multiline content mismatch:\ngot:  %q\nwant: %q", tc.Arguments["content"], expected)
	}
}

func TestCodeContainingQuotes(t *testing.T) {
	tr := New()
	input := `<｜｜DSML｜｜ calls>
<｜｜DSML｜｜ invoke name="create_file">
<｜｜DSML｜｜ parameter name="path" string="true">app.py</｜｜DSML｜｜ parameter>
<｜｜DSML｜｜ parameter name="content" string="true">name = "Alice"
print(f"Hello, {name}!")</｜｜DSML｜｜ parameter>
</｜｜DSML｜｜ invoke>
</｜｜DSML｜｜ calls>`

	resp, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tc := resp.ToolCalls[0]
	expected := "name = \"Alice\"\nprint(f\"Hello, {name}!\")"
	if tc.Arguments["content"] != expected {
		t.Errorf("quotes content mismatch:\ngot:  %q\nwant: %q", tc.Arguments["content"], expected)
	}
}

func TestCodeContainingXMLLikeText(t *testing.T) {
	tr := New()
	input := `<｜｜DSML｜｜ calls>
<｜｜DSML｜｜ invoke name="create_file">
<｜｜DSML｜｜ parameter name="path" string="true">template.html</｜｜DSML｜｜ parameter>
<｜｜DSML｜｜ parameter name="content" string="true"><div class="container">
  <p>Hello World</p>
</div></｜｜DSML｜｜ parameter>
</｜｜DSML｜｜ invoke>
</｜｜DSML｜｜ calls>`

	resp, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tc := resp.ToolCalls[0]
	expected := `<div class="container">
  <p>Hello World</p>
</div>`
	if tc.Arguments["content"] != expected {
		t.Errorf("XML content mismatch:\ngot:  %q\nwant: %q", tc.Arguments["content"], expected)
	}
}

func TestEmptyAssistantText(t *testing.T) {
	tr := New()
	input := `<｜｜DSML｜｜ calls>
<｜｜DSML｜｜ invoke name="list_dir">
<｜｜DSML｜｜ parameter name="path" string="true">.</｜｜DSML｜｜ parameter>
</｜｜DSML｜｜ invoke>
</｜｜DSML｜｜ calls>`

	resp, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "" {
		t.Errorf("expected empty content, got %q", resp.Content)
	}
}

func TestMalformedDSML(t *testing.T) {
	tr := New()

	// Missing closing invoke tag.
	_, err := tr.Translate(`<｜｜DSML｜｜ calls>
<｜｜DSML｜｜ invoke name="list_dir">
<｜｜DSML｜｜ parameter name="path" string="true">.</｜｜DSML｜｜ parameter>
</｜｜DSML｜｜ calls>`)
	if err == nil {
		t.Error("expected error for unclosed invoke, got nil")
	}

	// Missing closing parameter tag.
	_, err = tr.Translate(`<｜｜DSML｜｜ calls>
<｜｜DSML｜｜ invoke name="list_dir">
<｜｜DSML｜｜ parameter name="path" string="true">.
</｜｜DSML｜｜ invoke>
</｜｜DSML｜｜ calls>`)
	if err == nil {
		t.Error("expected error for unclosed parameter, got nil")
	}
}

func TestTruncatedDSML(t *testing.T) {
	tr := New()

	// Truncated: has opening calls but no closing.
	input := `I'll check the directory.

<｜｜DSML｜｜ calls>
<｜｜DSML｜｜ invoke name="list_dir">`
	resp, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should fall back to extracting text, stripping partial DSML.
	if resp.Content == "" {
		t.Error("expected some content from truncated input")
	}
}

func TestMultipleConsecutiveToolCalls(t *testing.T) {
	tr := New()
	input := `<｜｜DSML｜｜ calls>
<｜｜DSML｜｜ invoke name="read_file">
<｜｜DSML｜｜ parameter name="path" string="true">a.txt</｜｜DSML｜｜ parameter>
</｜｜DSML｜｜ invoke>
<｜｜DSML｜｜ invoke name="read_file">
<｜｜DSML｜｜ parameter name="path" string="true">b.txt</｜｜DSML｜｜ parameter>
</｜｜DSML｜｜ invoke>
<｜｜DSML｜｜ invoke name="read_file">
<｜｜DSML｜｜ parameter name="path" string="true">c.txt</｜｜DSML｜｜ parameter>
</｜｜DSML｜｜ invoke>
<｜｜DSML｜｜ invoke name="read_file">
<｜｜DSML｜｜ parameter name="path" string="true">d.txt</｜｜DSML｜｜ parameter>
</｜｜DSML｜｜ invoke>
<｜｜DSML｜｜ invoke name="read_file">
<｜｜DSML｜｜ parameter name="path" string="true">e.txt</｜｜DSML｜｜ parameter>
</｜｜DSML｜｜ invoke>
</｜｜DSML｜｜ calls>`

	resp, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.ToolCalls) != 5 {
		t.Fatalf("expected 5 tool calls, got %d", len(resp.ToolCalls))
	}
	for i, tc := range resp.ToolCalls {
		if tc.Name != "read_file" {
			t.Errorf("tool %d name = %q", i, tc.Name)
		}
		if tc.ID == "" {
			t.Errorf("tool %d has empty ID", i)
		}
	}
	// Verify unique IDs.
	ids := make(map[string]bool)
	for _, tc := range resp.ToolCalls {
		if ids[tc.ID] {
			t.Errorf("duplicate ID: %s", tc.ID)
		}
		ids[tc.ID] = true
	}
}

func TestToolWithEmptyContentParameter(t *testing.T) {
	tr := New()
	input := `<｜｜DSML｜｜ calls>
<｜｜DSML｜｜ invoke name="create_file">
<｜｜DSML｜｜ parameter name="path" string="true">empty.txt</｜｜DSML｜｜ parameter>
<｜｜DSML｜｜ parameter name="content" string="true"></｜｜DSML｜｜ parameter>
</｜｜DSML｜｜ invoke>
</｜｜DSML｜｜ calls>`

	resp, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tc := resp.ToolCalls[0]
	if tc.Arguments["path"] != "empty.txt" {
		t.Errorf("arguments[path] = %q", tc.Arguments["path"])
	}
	if tc.Arguments["content"] != "" {
		t.Errorf("expected empty content, got %q", tc.Arguments["content"])
	}
}

func TestTextBeforeAndAfterDSML(t *testing.T) {
	tr := New()
	input := `Let me check the files first.

<｜｜DSML｜｜ calls>
<｜｜DSML｜｜ invoke name="list_dir">
<｜｜DSML｜｜ parameter name="path" string="true">.</｜｜DSML｜｜ parameter>
</｜｜DSML｜｜ invoke>
</｜｜DSML｜｜ calls>

I'll review the results and make changes.`

	resp, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "Let me check the files first.\nI'll review the results and make changes."
	if resp.Content != expected {
		t.Errorf("content = %q, want %q", resp.Content, expected)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
}

func TestJSONSerialization(t *testing.T) {
	tr := New()
	input := `I'll check.

<｜｜DSML｜｜ calls>
<｜｜DSML｜｜ invoke name="git_status">

</｜｜DSML｜｜ invoke>
</｜｜DSML｜｜ calls>`

	resp, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var roundtrip TranslatedResponse
	if err := json.Unmarshal(data, &roundtrip); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if roundtrip.Content != resp.Content {
		t.Errorf("roundtrip content mismatch")
	}
	if len(roundtrip.ToolCalls) != len(resp.ToolCalls) {
		t.Errorf("roundtrip tool_calls mismatch")
	}
}

func TestUserExample(t *testing.T) {
	tr := New()
	input := "I'll check the installed Python version.\n\n<｜｜DSML｜｜ calls>\n<｜｜DSML｜｜ invoke name=\"run_command\">\n<｜｜DSML｜｜ parameter name=\"command\" string=\"true\">python --version</｜｜DSML｜｜ parameter>\n</｜｜DSML｜｜ invoke>\n</｜｜DSML｜｜ calls>"
	resp, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("Content: %q", resp.Content)
	t.Logf("ToolCalls: %d", len(resp.ToolCalls))
	for i, tc := range resp.ToolCalls {
		t.Logf("  [%d] ID=%s Name=%s Args=%v", i, tc.ID, tc.Name, tc.Arguments)
	}
	if resp.Content != "I'll check the installed Python version." {
		t.Errorf("content = %q", resp.Content)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "run_command" {
		t.Errorf("name = %q", resp.ToolCalls[0].Name)
	}
	if resp.ToolCalls[0].Arguments["command"] != "python --version" {
		t.Errorf("command = %q", resp.ToolCalls[0].Arguments["command"])
	}
}

func TestEmptyInput(t *testing.T) {
	tr := New()
	resp, err := tr.Translate("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "" {
		t.Errorf("expected empty content, got %q", resp.Content)
	}
	if len(resp.ToolCalls) != 0 {
		t.Errorf("expected 0 tool calls, got %d", len(resp.ToolCalls))
	}
}

func TestWhitespaceOnly(t *testing.T) {
	tr := New()
	resp, err := tr.Translate("   \n  \n  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "" {
		t.Errorf("expected empty content, got %q", resp.Content)
	}
}

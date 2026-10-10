<div align="center">

<table align="center" border="0" cellspacing="0" cellpadding="0">
  <tr>
    <td align="center" valign="middle" style="padding-right: 16px;">
      <img src="readmeicon.png" alt="NightCode logo" width="95" />
    </td>
    <td align="left" valign="middle">
      <span style="font-size: 52px; font-weight: 800; letter-spacing: 1px; color: #ffffff; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;">NIGHTCODE</span>
    </td>
  </tr>
</table>

<p><strong>An AI-powered coding agent with a chat interface. It reads, writes, edits, searches and runs code in your workspace, and every change can be undone.</strong></p>

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)
![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black)
![TypeScript](https://img.shields.io/badge/TypeScript-6-3178C6?logo=typescript&logoColor=white)
![SQLite](https://img.shields.io/badge/SQLite-pure--Go-003B57?logo=sqlite&logoColor=white)

[Screenshots](#screenshots) · [Features](#features) · [Quick start](#quick-start) · [Configuration](#configuration) · [Architecture](#architecture) · [Providers](#llm-providers) · [Tools](#built-in-tools) · [MCP](#mcp-connectors) · [Roadmap](#roadmap)

</div>

---

## Overview

NightCode is a full-stack agentic coding assistant, in the spirit of Claude Code or Cursor. You describe a task in a chat window, and an agent works through it step by step: it reads your files, searches the codebase, edits code, runs shell commands, and reports back, while you watch each tool call stream in live.

It is built from two parts:

- **Backend (Go):** runs the agent loop, talks to LLM providers, executes sandboxed tools, and stores everything in SQLite.
- **Frontend (React + TypeScript):** a polished chat UI with streaming markdown, a tool-execution timeline, an artifact panel, and workspace management.

The two communicate over HTTP and **Server-Sent Events (SSE)**, so responses and tool activity appear in real time.

## Screenshots

<div align="center">
<img src="image%20(3).png" alt="NightCode home page" width="100%" />
<br/><sub><b>Home:</b> start a new chat and pick a model</sub>

</div>

<br/>

<table>
  <tr>
    <td width="50%">
      <img src="image%20(4).png" alt="Chat with streaming response" />
      <br/><sub><b>Chat:</b> streaming responses with markdown and syntax highlighting</sub>
    </td>
    <td width="50%">
      <img src="image%20(5).png" alt="Tool execution timeline" />
      <br/><sub><b>Tool timeline:</b> watch every file read, edit and command as it happens</sub>
    </td>
  </tr>
  <tr>
    <td width="50%">
      <img src="image%20(6).png" alt="Artifact panel" />
      <br/><sub><b>Artifacts:</b> view, copy and download generated code</sub>
    </td>
    <td width="50%">
      <img src="image%20(7).png" alt="Workspaces page" />
      <br/><sub><b>Workspaces:</b> manage separate project folders</sub>
    </td>
  </tr>
  <tr>
    <td width="50%">
      <img src="image%20(8).png" alt="MCP connectors page" />
      <br/><sub><b>Connectors:</b> add and toggle MCP tool servers</sub>
    </td>
    <td width="50%">
      <img src="image%20(9).png" alt="Settings page" />
      <br/><sub><b>Settings:</b> appearance and account options</sub>
    </td>
  </tr>
</table>

## Features

**Agent**
- ReAct-style loop (reason, act, observe) with a 20-iteration cap
- Concurrent tool execution with a consecutive-failure cap
- Cancel a running turn at any time
- Automatic retry status with countdown on rate limits

**Safety and control**
- All file operations are confined to a workspace root, with symlink resolution and path-traversal protection
- **Undo**: write and edit operations store reversal data, so you can roll back the changes from any message
- Permission modes in the UI: Read Only, Auto, Full Access

**Models**
- Multiple LLM providers behind one interface, switchable at runtime, with per-model client caching
- Model picker in the prompt input, populated from your configured API keys

**Context management**
- 128K token budget with a reserve for responses
- Automatic history compaction when over budget
- Prioritized system-prompt truncation (file tree, skills, instructions)
- Project instructions via `.nightcode/instructions.md`
- Reusable skills via `.nightcode/skills/*.md`
- Workspace file tree and git status injected into the prompt

**Extensibility**
- **MCP (Model Context Protocol)** connectors over stdio: add external tool servers from the UI
- Includes a Windows-control MCP server (open apps, type text, scroll, screenshot and more)

**Interface**
- Streaming markdown with Shiki syntax highlighting
- Collapsible tool timeline with human-readable summaries
- Artifact panel for generated code and documents (copy, download, fullscreen)
- File attachments (images and text files)
- Dark, light and system themes
- Workspaces, chat history, and connector management pages

## Quick start

### Prerequisites

| Tool | Version | Notes |
|---|---|---|
| Go | 1.26+ | See `Backend/go.mod` for the exact version |
| Node.js | 20+ | For the frontend |
| pnpm | latest | The repo ships a `pnpm-lock.yaml` |
| Git | any | Used by the git tools and context |
| Python | 3.10+ | Optional, only for the Windows-control MCP server |
| ripgrep | any | Optional, speeds up `grep`. A native Go fallback is used if missing |

### 1. Clone the repository

```bash
git clone https://github.com/<your-username>/NightCode.git
cd NightCode
```

### 2. Configure the backend

```bash
cd Backend
cp .env.example .env      # on Windows: copy .env.example .env
```

Open `.env` and set a provider and its API key. The quickest way to try NightCode without any key is the built-in echo agent:

```env
NIGHTCODE_PROVIDER=echo
```

For a real model, see [Configuration](#configuration) and [LLM providers](#llm-providers).

> **Never commit `.env`.** It is listed in `.gitignore`. Only `.env.example` (with empty values) belongs in version control.

### 3. Run the backend

```bash
cd Backend
go run ./cmd/server
```

The API starts on `http://localhost:3001`.

### 4. Run the frontend

```bash
cd Frontend
pnpm install
pnpm dev
```

Open `http://localhost:5173`.

### One-command start (Windows)

After building the backend once (`go build -o server.exe ./cmd/server` inside `Backend/`), you can start both services with:

```powershell
.\start.ps1
```

or

```bat
start.cmd
```

## Configuration

### Backend environment variables

| Variable | Description | Default |
|---|---|---|
| `NIGHTCODE_PORT` | Server port | `3001` |
| `NIGHTCODE_DB_PATH` | SQLite database path | `nightcode.db` |
| `NIGHTCODE_CORS_ORIGIN` | Allowed frontend origin | `http://localhost:5173` |
| `NIGHTCODE_PROVIDER` | Default provider (see below) | `echo` |
| `NIGHTCODE_WORKSPACES_ROOT` | Base directory for workspace folders | `workspaces/` |
| `NIGHTCODE_DEBUG_TOOLS` | Enable debug tool endpoints (`true`/`false`) | off |
| `NIGHTCODE_STREAM_DELAY_MS` | Debug: slow down streamed text | `0` |

Provider-specific variables:

| Provider | Variables |
|---|---|
| Gemini | `GEMINI_API_KEY`, `GEMINI_MODEL` |
| Groq | `GROQ_API_KEY`, `GROQ_MODEL`, `GROQ_BASE_URL` |
| OpenCode Zen | `OPENCODE_API_KEY`, `OPENCODE_MODEL`, `OPENCODE_BASE_URL` |
| OpenRouter | `OPENROUTER_API_KEY` (also `_2`, `_3` for key rotation), `OPENROUTER_MODEL`, `OPENROUTER_BASE_URL` |
| Calabrass (local) | `CALABRASS_MODEL`, `CALABRASS_BASE_URL` |
| Cloudflare Workers AI | `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_MODEL`, `CLOUDFLARE_BASE_URL` |
| AshnaAI | `ASHNAAI_API_KEY`, `ASHNAAI_MODEL`, `ASHNAAI_BASE_URL` |
| NightCode gateway (local) | `NIGHTCODE_LOCAL_API_KEY`, `NIGHTCODE_LOCAL_MODEL`, `NIGHTCODE_LOCAL_BASE_URL` |

### Frontend environment variables

Create `Frontend/.env.local` if you need to override these:

| Variable | Description | Default |
|---|---|---|
| `VITE_API_URL` | Backend base URL | `http://localhost:3001` |
| `VITE_USE_MOCK` | Use the built-in mock runtime instead of the backend (`true`) | off |

### Project instructions and skills

Place these inside any workspace to customize the agent for that project:

```
.nightcode/
├── instructions.md      # project-wide rules the agent always sees
└── skills/
    └── *.md             # reusable skills, listed in the system prompt
```

## Architecture

```
┌──────────────────┐      HTTP + SSE      ┌──────────────────────┐
│     Frontend     │ <------------------> │       Backend        │
│  React 19 + Vite │        :3001         │          Go          │
│      :5173       │                      │                      │
└──────────────────┘                      └──────────┬───────────┘
        │                                            │
        │ localStorage                               │ SQLite
        v                                            v
  Browser storage                              nightcode.db
```

The backend follows a ports-and-adapters layout:

| Layer | Path | Responsibility |
|---|---|---|
| API | `internal/api` | HTTP handlers, SSE streaming, CORS, connectors, undo |
| Agent | `internal/agent` | ReAct loop, tool execution, error classification |
| Provider | `internal/provider` | Pluggable LLM providers behind one interface |
| Context | `internal/context` | Prompt assembly, token budgeting, history compaction |
| Tools | `internal/tools` | Sandboxed file, search, shell and git tools |
| MCP | `internal/mcp` | JSON-RPC 2.0 stdio client for external tool servers |
| Store | `internal/store` | SQLite persistence |

### How a turn works

```
User message
   -> context builder (system prompt + history + file tree + git status)
   -> provider StreamChat
   -> text deltas streamed to the UI
   -> tool calls executed concurrently
   -> results fed back to the model
   -> repeat until no more tool calls (max 20 iterations)
   -> final response saved to SQLite
```

### SSE events

The frontend renders a turn from a stream of events, including:

`turn.started` → `assistant.delta` → `tool.started` → `tool.completed` / `tool.failed` → `turn.completed`

plus events for artifacts, retries, and errors (12 event types in total, defined in `Frontend/src/types/events.ts` and `Backend/internal/types/types.go`).

## LLM providers

| Provider | Notes |
|---|---|
| Google Gemini | Via the Eino framework |
| Groq | OpenAI-compatible |
| OpenCode Zen | OpenAI-compatible |
| OpenRouter | Supports multiple keys with rotation |
| Calabrass | Local server, no auth |
| Cloudflare Workers AI | OpenAI-compatible |
| AshnaAI | OpenAI-compatible |
| NightCode gateway | Local LLM gateway over raw HTTP SSE |
| Echo | Placeholder agent for testing, needs no key |

Default models are set in each provider file under `Backend/internal/provider/` and can be overridden with the `*_MODEL` variables above. A provider is added by implementing the interface in `provider.go`. OpenAI-compatible APIs can reuse the shared helpers in `openai_compat.go`.

## Built-in tools

| Tool | What it does |
|---|---|
| `read_file` | Read a file, with optional line ranges (capped at 2000 lines / 100 KB) |
| `write_file` | Create or overwrite a file, creating parent directories as needed |
| `edit_file` | Find-and-replace edit, returns a unified diff |
| `list_dir` | List a directory, flat or as a tree, honoring ignore patterns |
| `grep` | Regex search via ripgrep, with a native Go fallback |
| `glob` | Find files by glob pattern |
| `shell` | Run a shell command (PowerShell on Windows) |
| `git_status` | Branch and short status |
| `git_diff` | Unified diff for a file or the whole tree |

`write_file` and `edit_file` record reversal data, which powers the **Reverse changes** button on each message.

## MCP connectors

NightCode can load external tool servers that speak the Model Context Protocol over stdio.

1. Open the **Connectors** page in the UI.
2. Add a connector with a name and a command, for example:
   ```
   python Backend/MCPs/win-control-mcp.py
   ```
3. Toggle it on. Its tools appear to the agent alongside the built-ins.

Connectors load in the background with a retry loop, so a slow server does not block startup.

**Windows-control MCP server** (`Backend/MCPs/win-control-mcp.py`) exposes 12 tools such as opening apps, typing text, scrolling and taking screenshots. It needs:

```bash
pip install fastmcp pyautogui pyperclip
```

## API reference

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/workspaces/{ws}/chats/{chat}/messages` | Send a message and stream the response (SSE) |
| `DELETE` | `/api/workspaces/{ws}/chats/{chat}/runs/current` | Cancel the current run |
| `DELETE` | `/api/workspaces/{ws}/chats/{chat}/runs/{runId}` | Cancel a specific run |
| `GET` | `/api/workspaces/{ws}/chats` | List chats |
| `GET` | `/api/workspaces/{ws}/chats/{chat}/messages` | List messages |
| `GET` | `/api/workspaces/{ws}/chats/{chat}/artifacts` | List artifacts |
| `POST` | `/api/workspaces/{ws}/chats/{chat}/messages/{id}/reverse` | Undo file changes from a message |
| `GET` | `/api/workspaces` | List workspaces |
| `POST` | `/api/workspaces` | Create a workspace |
| `DELETE` | `/api/workspaces/{ws}` | Delete a workspace |
| `GET` | `/api/models` | List models for configured providers |
| `GET` | `/api/connectors` | List MCP connectors |
| `POST` | `/api/connectors` | Create a connector |
| `DELETE` | `/api/connectors/{id}` | Delete a connector |
| `PATCH` | `/api/connectors/{id}/toggle` | Enable or disable a connector |

When `NIGHTCODE_DEBUG_TOOLS=true`, `/api/debug/tools` endpoints are also available for running tools directly. Leave this off outside local development.

## Project structure

```
NightCode/
├── Backend/
│   ├── cmd/server/main.go       # entry point
│   ├── internal/
│   │   ├── api/                 # HTTP handlers, connectors, undo
│   │   ├── agent/               # agent loop
│   │   ├── context/             # prompt + token budget
│   │   ├── mcp/                 # MCP client and adapter
│   │   ├── provider/            # LLM providers
│   │   ├── store/               # SQLite layer
│   │   └── tools/               # built-in tools
│   ├── MCPs/                    # bundled MCP servers
│   └── workspaces/              # workspace folders
├── Frontend/
│   └── src/
│       ├── components/          # chat view, prompt input, timeline, panels
│       ├── context/             # chat state
│       ├── lib/                 # SSE client, tool renderers, markdown
│       ├── pages/               # home, workspaces, connectors, settings
│       └── types/               # event and message types
├── start.cmd                    # Windows launcher
├── start.ps1                    # PowerShell launcher
└── LICENSE
```

## Tech stack

**Backend:** Go, `net/http` with SSE, `modernc.org/sqlite` (pure Go, no CGO), `cloudwego/eino`, `google.golang.org/genai`, `doublestar` for globbing, `godotenv`

**Frontend:** React 19, TypeScript, Vite, TanStack Router, Tailwind CSS 4, shadcn/ui with Base UI, Shiki, unified/remark, lucide-react

**MCP server:** FastMCP, pyautogui, pyperclip

## Security notes

NightCode is designed as a **local, single-user tool**. Please keep this in mind:

- The `shell` tool runs real commands on your machine. Only run NightCode on a machine and workspace you trust.
- There is currently **no authentication** on the API. Do not expose port `3001` to a network. Run it on `localhost` only.
- File tools are sandboxed to the workspace root, but the shell and MCP connectors are not sandboxed.
- Keep API keys in `Backend/.env` only, and never commit that file.
- Review any MCP connector command before enabling it, since it runs as a local process.

## Development

```bash
# Backend
cd Backend
go build ./...
go test ./...

# Frontend
cd Frontend
pnpm dev         # dev server
pnpm build       # type-check + production build
pnpm lint        # ESLint
pnpm format      # Prettier
pnpm typecheck   # TypeScript only
```

To work on the UI without a backend, set `VITE_USE_MOCK=true`. The mock runtime replays scripted scenarios.

## Roadmap

- [ ] Nebius Token Factory provider with NVIDIA Nemotron models
- [ ] Model routing: larger models for planning, faster ones for routine calls
- [ ] Automatic test-run-fix loop
- [ ] Authentication and a locked-down hosted demo mode
- [ ] Cross-platform shell tool and Docker setup
- [ ] Functional Settings page (profile, API keys, appearance)
- [ ] Broader test coverage (store, handlers, tools, frontend)
- [ ] Goroutine-leak and SQLite locking fixes

## Contributing

Contributions are welcome.

1. Fork the repository and create a branch: `git checkout -b feat/your-feature`
2. Make your changes, with tests where it makes sense
3. Run `go test ./...` and `pnpm lint` and `pnpm typecheck`
4. Open a pull request describing what changed and why

Please never include real API keys or `.env` files in a pull request.

## License

Released under the [MIT License](LICENSE). Copyright 2026 Mubasher Chaudhary.

# M.A.N.A — Multi-Agent Network Architecture

M.A.N.A is a terminal-native, highly concurrent AI orchestration system. It provides a single, unified interface to converse with multiple specialized AI agents simultaneously, in real time, without leaving the terminal.

Rather than wrapping a single chatbot, M.A.N.A treats agents as peers. It routes messages with precision, orchestrates concurrent fan-out streams, and renders responses side-by-side using a purpose-built Bubble Tea terminal UI.

## See it in Action

Demo Video: [M.A.N.A Demo on YouTube](https://www.youtube.com/watch?v=eFaEFUDQHzA&utm_source=chatgpt.com)

---

# Architecture

M.A.N.A is built entirely in Go and split into two clean layers communicating over WebSockets:

* **The Proxy Router (mana-server):** A concurrent WebSocket hub that owns routing intelligence, heartbeat monitoring, upstream agent communication, and process lifecycle management via `os/exec`.

* **The TUI Client:** A Bubble Tea frontend utilizing Lip Gloss and Glamour for real-time Markdown rendering, viewport management, and keyboard-first interaction.

Neither layer knows more about the other than it needs to. The proxy handles the channel-based async stream merging, while the TUI handles the terminal aesthetics.

---

# Core Features

## Concurrent Fan-Out

Talk to multiple agents at once. The proxy spawns lightweight goroutines for each agent and streams their responses fully interleaved. The slowest agent never blocks the fastest.

## Inline @mention Parsing

Target specific agents within a single message (e.g., `@airi analyze this while @zephyr checks the logs`). M.A.N.A intelligently splits the payload and prepends context.

## Active Heartbeat Monitoring

The server continuously probes agent WebSocket endpoints in the background. Use `/online` to instantly view the real-time network status without blocking.

## Cold-Start Process Waking

If an agent is offline, use `/wake <agent>` or `/wake all` to spawn their underlying server processes directly from the TUI.

## Data-Driven Registry

Add new agents by adding a few lines to a `config.yaml` file. No code changes required. The TUI's autocomplete and routing layers update automatically.

## Terminal-First Aesthetics

Beautiful, rounded-border response boxes that grow dynamically as chunks arrive, complete with context-aware autocomplete, file attachments (`ctrl+f`), and voice input (`arecord`).

---

# Getting Started

## Prerequisites

* Go 1.22 or higher
* Ubuntu / Linux / macOS environment (recommended for process management)

## Installation

Clone the repository:

```bash
git clone https://github.com/Dpaste20/MANA.git
cd mana/mana-server
```

Build the proxy server:

```bash
go build -o mana-server
```

---

# Configuration

M.A.N.A requires a `config.yaml` file in the directory where the server is run. This file acts as the ultimate source of truth for the network.

```yaml
agents:
  airi:
    display_name: Airi
    ws_url: ws://localhost:8000/ws/chat
    start_cmd: ".venv/bin/python server.py"
    work_dir: "~/Projects/Airi_cli"

  zephyr:
    display_name: Zephyr
    ws_url: ws://localhost:8004/ws/chat
    start_cmd: ".venv/bin/python ZephyrServer.py"
    work_dir: "~/Projects/Zephyr"
```

### Field Descriptions

* `ws_url`: The WebSocket endpoint the agent listens on.
* `start_cmd`: (Optional) The shell command used by `/wake` to boot the agent.
* `work_dir`: (Optional) The directory to execute the `start_cmd` from.

---

# Running the System

Start the proxy server:

```bash
./mana-server
```

The server will boot, parse `config.yaml`, and begin its heartbeat cycle on `ws://0.0.0.0:8080/ws/chat`.

Next, start your TUI client in a separate terminal to connect to the hub.

---

# TUI Commands

The interface relies on slash commands for orchestration:

| Command           | Description                                             |
| ----------------- | ------------------------------------------------------- |
| `/talk <agent>`   | Set the default agent for all subsequent messages       |
| `/talk <a1> <a2>` | Fan-out subsequent messages to multiple agents          |
| `/talk all`       | Broadcast messages to the entire network                |
| `/online`         | View the live heartbeat status of all registered agents |
| `/wake <agent>`   | Spawn the underlying process for an offline agent       |
| `/wake all`       | Boot all configured agents in parallel                  |
| `/attach <path>`  | Stage a text or PDF file for the next payload           |

## Shortcuts

| Shortcut | Action                          |
| -------- | ------------------------------- |
| `ctrl+f` | Open interactive file picker    |
| `ctrl+d` | Clear staged attachments        |
| `ctrl+y` | Copy last response to clipboard |
| `Space`  | Toggle voice recording          |

---

# Agent Integration Contract

Integrating an external agent requires zero changes to the M.A.N.A codebase. An agent merely needs to expose a WebSocket endpoint that accepts a JSON request and streams back JSON responses containing a `type` field (e.g., `chunk`, `end`, `error`).

M.A.N.A is framework-agnostic. Whether your agent uses LangChain, LlamaIndex, raw FastAPI, or a local Rust model server, it can seamlessly join the network.

---


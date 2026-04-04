# Architecture

Last verified: 2026-04-04

This document describes the current `way-island` architecture as implemented in the repository today.

## Goal

`way-island` is a small Wayland overlay that watches coding-agent activity, shows compact session state in a GTK layer-shell UI, and can jump focus back to the tmux pane and terminal window that owns a session.

The current implementation supports two hook producers:

- Claude Code
- Codex

Those producers are normalized into one internal session model.

## Top-Level Flow

```text
Claude Code hook / Codex hook
    -> way-island hook
        -> payload normalization
        -> event mapping
        -> Unix socket message
            -> SessionManager
                -> SessionUpdate stream
                    -> overlayModel
                    -> approval detector (Codex/Bash only)
                    -> GTK UI payload
                        -> layer-shell pill
                        -> expanded session list
```

Terminal focus runs as a separate path:

```text
session row click
    -> sessionFocuser
        -> resolve host agent PID
        -> resolve tmux pane
        -> switch tmux client to pane
        -> focus Wayland toplevel by app_id + title
```

## Main Components

### 1. Hook ingress

Implemented in [`cli.go`](/home/katsuma/src/github.com/ka2n/way-island/cli.go).

`way-island hook` reads JSON from stdin, detects whether the payload came from Claude Code or Codex, normalizes the payload shape, attaches local process metadata, and forwards one socket message to the daemon.

Normalization responsibilities:

- choose hook source: Claude vs Codex
- map external event names into internal event names
- normalize Codex `tool_name` / `tool_input.command`
- attach `_ppid`, PID namespace inode, tty, and jail metadata
- attach `_hook_source` so downstream logic can branch safely by producer

### 2. Socket transport

Implemented in [`internal/socket/server.go`](/home/katsuma/src/github.com/ka2n/way-island/internal/socket/server.go) and [`internal/socket/client.go`](/home/katsuma/src/github.com/ka2n/way-island/internal/socket/client.go).

The daemon listens on a Unix domain socket under `XDG_RUNTIME_DIR`. Hook invocations are intentionally short-lived and stateless; the daemon owns session state.

The server also exposes `_inspect` so local state can be queried without reaching into process memory directly.

### 3. Session state model

Implemented in [`internal/socket/session_manager.go`](/home/katsuma/src/github.com/ka2n/way-island/internal/socket/session_manager.go).

The session manager is the canonical runtime model for active sessions. It converts incoming socket messages into `SessionUpdate`s and applies timeouts.

Current session fields include:

- identity: `ID`, `DisplayName`
- state: `State`
- source metadata: `HookSource`
- tool metadata: `CurrentTool`, `CurrentAction`
- timing: `LastEventAt`
- focus metadata: `AgentPID`, namespace inode, start time, tty fields
- jail metadata: `AgentInJail`

Current internal states:

- `idle`
- `working`
- `tool_running`
- `waiting`

For Codex specifically, [`internal/socket/session_manager.go`](/home/katsuma/src/github.com/ka2n/way-island/internal/socket/session_manager.go) adjusts a root-session `hook:idle` update to `waiting`, while subagent `hook:idle` remains `idle`.

### 4. Overlay store

Implemented in [`overlay_model.go`](/home/katsuma/src/github.com/ka2n/way-island/overlay_model.go).

`overlayModel` is the UI-facing snapshot store. It receives `SessionUpdate`s and produces a base64-encoded TSV payload for the GTK layer.

Current payload columns:

1. session id
2. display name
3. state
4. current action

### 5. UI update pipeline

Implemented in [`ui_updates.go`](/home/katsuma/src/github.com/ka2n/way-island/ui_updates.go).

This layer throttles flushes to avoid over-updating GTK while keeping the latest session state.

Pipeline details:

- incoming `SessionUpdate`s are applied immediately to the store
- UI flushes are throttled
- synthetic updates from the approval detector are merged into the same stream

### 6. Approval detector

Implemented in [`approval_detector.go`](/home/katsuma/src/github.com/ka2n/way-island/approval_detector.go).

This is intentionally downstream of event mapping.

Reason:

- Codex does not currently emit a dedicated approval-request hook event
- `PreToolUse` alone cannot distinguish "normal long-running command" from "approval prompt is currently blocking"
- the detector needs delayed observation plus tmux pane inspection

Current behavior:

- runs only for `HookSource == "codex"`
- routes by `CurrentTool`
- currently only the `bash` pipeline is implemented
- waits 2 seconds after `tool_running`
- uses `tmux capture-pane` on the resolved pane
- if approval prompt markers are found, emits a synthetic `waiting` update

This keeps the architecture split into:

- deterministic event mapping
- stateful inference pipelines

### 7. tmux + focus path

Implemented in [`focus.go`](/home/katsuma/src/github.com/ka2n/way-island/focus.go), [`proc.go`](/home/katsuma/src/github.com/ka2n/way-island/proc.go), and [`wayland_toplevel.go`](/home/katsuma/src/github.com/ka2n/way-island/wayland_toplevel.go).

Current focus algorithm:

1. resolve session by id
2. resolve host PID from namespaced agent metadata
3. resolve the owning tmux pane
4. focus the tmux client onto that pane
5. focus the Wayland terminal toplevel by `app_id` and window title

The same pane resolution logic is reused by the Codex approval detector to inspect pane contents.

## Source-Specific Pipelines

### Claude Code

```text
Claude hook payload
    -> parseClaudeHookPayload
    -> event mapping
    -> SessionManager
    -> overlayModel / UI
```

Important current rule:

- Claude sessions do not enter the tmux approval detector pipeline

Claude already has a real `Notification -> waiting` event mapping, so it does not need the Codex fallback inference.

### Codex

```text
Codex hook payload
    -> parseCodexHookPayload
    -> event mapping
    -> SessionManager
    -> overlayModel / UI
    -> tool-specific approval detector pipeline
```

Important current rule:

- only Codex `bash` tool sessions can be upgraded from `tool_running` to synthetic `waiting`

## Why Event Mapping And Approval Detection Are Separate

The split is deliberate.

Event mapping is for immediate, deterministic translation from hook payload to internal event:

- `PreToolUse -> tool_start`
- `PostToolUse -> tool_end`
- `Notification -> waiting`
- `Stop -> idle`

Approval detection is not deterministic from the hook payload alone. It depends on:

- hook source
- current tool
- elapsed time
- tmux pane contents

There is one additional session-layer adjustment after mapping:

- Codex root-session `idle` is reclassified to `waiting` once subagent metadata has been considered

If those concerns were mixed into event mapping, `PreToolUse` would become producer-specific and heuristic-heavy, which would make the core hook ingest path harder to reason about.

## Current Constraints

- JSONL parsing currently targets Claude transcript shape only
- Codex approval waiting is inferred, not explicitly emitted
- approval detection is tmux-specific
- current Wayland focusing is tuned to terminal toplevel matching by title
- GTK build/runtime verification still depends on the project dev shell

## Related Documents

- Claude Code specifics: [`docs/agents/claude-code.md`](/home/katsuma/src/github.com/ka2n/way-island/docs/agents/claude-code.md)
- Codex specifics: [`docs/agents/codex.md`](/home/katsuma/src/github.com/ka2n/way-island/docs/agents/codex.md)

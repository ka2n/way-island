# Codex Integration

Last verified: 2026-04-04

This file summarizes the Codex hook behavior that `way-island` currently depends on, plus the gaps that require local inference.

## Official References

- Codex hooks: https://developers.openai.com/codex/hooks
- Codex config reference: https://developers.openai.com/codex/config-reference
- Codex open source config notes: https://github.com/openai/codex/blob/main/docs/config.md

## Relevant External Hook Surface

The current official Codex hooks documentation describes:

- hooks are experimental
- hooks are behind `[features].codex_hooks = true`
- hooks are discovered from `~/.codex/hooks.json` and `<repo>/.codex/hooks.json`
- `PreToolUse`, `PostToolUse`, `UserPromptSubmit`, and `Stop` are turn-scoped

Current documented hook events used by `way-island`:

- `SessionStart`
- `SessionEnd`
- `UserPromptSubmit`
- `PreToolUse`
- `PostToolUse`
- `Stop`

The current docs also explicitly state that:

- `PreToolUse` currently only supports Bash tool interception
- `PostToolUse` currently only supports Bash tool results
- matcher support is limited, and `PreToolUse` / `PostToolUse` currently emit only `Bash`

## Important Gap

As of the verification date above, Codex does not expose a dedicated approval-request hook event in the official hook surface used here.

That is the key difference from Claude Code.

Practical consequence:

- `PreToolUse` tells us Codex is about to run `Bash`
- it does not tell us whether execution will proceed immediately or stop on an approval prompt

This is why `way-island` cannot map Codex approvals directly in the hook event mapping layer.

## What `way-island` Uses

Current local mapping in [`cli.go`](/home/katsuma/src/github.com/ka2n/way-island/cli.go):

- `SessionStart -> session_start`
- `SessionEnd -> session_end`
- `UserPromptSubmit -> working`
- `PreToolUse -> tool_start`
- `PostToolUse -> tool_end`
- `Stop -> idle`

Current runtime adjustment in [`internal/socket/session_manager.go`](/home/katsuma/src/github.com/ka2n/way-island/internal/socket/session_manager.go):

- `hook:idle` for a root Codex session is surfaced as `waiting`
- `hook:idle` for a detected subagent remains `idle`

Practical consequence:

- the hook mapping layer still treats `Stop` as `idle`
- the session layer reclassifies root Codex sessions to `waiting` because that state usually means the main agent is ready for the next user input rather than fully inactive

During normalization, `parseCodexHookPayload` also extracts:

- `tool_name`
- `tool_input.command`

and stores normalized fields:

- `tool`
- `command`

Additional local metadata attached by `way-island hook`:

- `_ppid`
- `_agent_pid_ns_inode`
- `_agent_start_time`
- `_agent_tty`
- `_agent_tty_nr`
- `_hook_tty`
- `_jai_jail`
- `_hook_source = "codex"`

## Subagent Detection

Codex session IDs are opaque identifiers. From hooks alone, `way-island`
cannot determine whether a session is a main agent session or a subagent
session.

That limitation applies specifically to the hook payloads handled by
[`cli.go`](/home/katsuma/src/github.com/ka2n/way-island/cli.go):

- a session ID by itself is not enough to classify a session
- the current hook surface does not include a documented `is_subagent` or
  `parent_session_id` field

However, Codex session transcript files contain richer metadata than the hook
stream.

Observed local path layout:

- `~/.codex/sessions/YYYY/MM/DD/rollout-<timestamp>-<session-id>.jsonl`

Observed `session_meta` fields for spawned subagents include:

- `forked_from_id`
- `source.subagent.thread_spawn.parent_thread_id`
- `source.subagent.thread_spawn.depth`
- `agent_nickname`
- `agent_role`

Practical consequence:

- hook-only classification is heuristic at best
- transcript-backed classification can identify subagents much more reliably

Example shape seen locally:

```json
{
  "type": "session_meta",
  "payload": {
    "id": "<child-session-id>",
    "forked_from_id": "<parent-session-id>",
    "source": {
      "subagent": {
        "thread_spawn": {
          "parent_thread_id": "<parent-session-id>",
          "depth": 1
        }
      }
    }
  }
}
```

The parent session transcript may also contain spawn-side events such as
`collab_agent_spawn_end` with `new_thread_id`, which can be used as a secondary
cross-check.

## Recommended Local Strategy

For Codex, the practical design is:

1. accept hook events as the low-latency source of session lifecycle and state
2. when a new session ID is first seen, look for the corresponding Codex
   session JSONL file
3. read only the initial `session_meta` entry
4. if `forked_from_id` or `source.subagent.thread_spawn.parent_thread_id`
   exists, mark the session as a subagent and record the parent session ID

This keeps the live path lightweight while still using the richer transcript
metadata when it becomes available.

## Hook-Only Fallback

Without transcript access, `way-island` can still observe the agent-process
identity attached by the local hook wrapper:

- `_ppid`
- `_agent_pid_ns_inode`
- `_agent_start_time`
- `_agent_tty`

Matching these fields can support a `same agent instance` heuristic, but that
still does not prove parent/child direction.

## Current Behavior In This Repository

Codex sessions go through this pipeline:

```text
Codex hook payload
    -> parseCodexHookPayload
    -> event mapping
    -> SessionManager
    -> overlayModel / UI
    -> tool-specific approval detector
```

Session fields used for Codex-specific routing:

- `HookSource = "codex"`
- `CurrentTool`
- `CurrentAction`
- `IsSubagent`

`CurrentTool` is normalized from `tool` / `tool_name`.

`CurrentAction` is built from:

- `tool_name`
- `command`

Example:

```text
Bash: go test -tags gtk4 ./...
```

Turn-end behavior for Codex sessions:

- root session `Stop` hooks are shown as `waiting`
- subagent `Stop` hooks are shown as `idle`

## Tool-Specific Approval Pipelines

The approval detector is intentionally tool-specific.

Current implementation:

- only `bash` has a pipeline
- unsupported tools are ignored
- Claude sessions are ignored entirely

Current `bash` approval pipeline:

1. wait for `tool_running` to remain active for 2 seconds
2. resolve the owning tmux pane for the session
3. run `tmux capture-pane -p`
4. look for approval prompt markers
5. if matched, emit a synthetic `waiting` state update

This is implemented in [`approval_detector.go`](/home/katsuma/src/github.com/ka2n/way-island/approval_detector.go).

## Why The Detector Is Not In Event Mapping

Codex approval detection is a stateful inference step, not a direct event translation.

It depends on:

- hook source being Codex
- current tool being `bash`
- elapsed time
- tmux pane text

If this were done in event mapping, `PreToolUse` would become a heuristic event instead of a deterministic translation.

## Known Constraints

- approval detection is heuristic and tmux-specific
- it depends on Codex TUI prompt wording
- it only covers `bash` today
- if Codex adds a real approval-request event, this fallback should probably be simplified or removed

## Upstream Tracking

These open issues are directly relevant to the current workaround:

- `openai/codex#15311` Add blocking PermissionRequest hook for external approval UIs
- `openai/codex#16301` add permission request event for parity with Claude Code
- `openai/codex#16484` machine-readable event surface for approvals and turn lifecycle

These are also referenced in the code near the current workaround in [`internal/socket/session_manager.go`](/home/katsuma/src/github.com/ka2n/way-island/internal/socket/session_manager.go).

## Implementation Pointers

- hook ingress: [`cli.go`](/home/katsuma/src/github.com/ka2n/way-island/cli.go)
- session state: [`internal/socket/session_manager.go`](/home/katsuma/src/github.com/ka2n/way-island/internal/socket/session_manager.go)
- approval inference: [`approval_detector.go`](/home/katsuma/src/github.com/ka2n/way-island/approval_detector.go)
- tmux pane capture: [`focus.go`](/home/katsuma/src/github.com/ka2n/way-island/focus.go)

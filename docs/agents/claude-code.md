# Claude Code Integration

Last verified: 2026-04-04

This file summarizes the Claude Code behavior that `way-island` currently depends on, plus how that behavior is mapped into the local runtime model.

## Official References

- Anthropic Claude Code hooks reference: https://docs.anthropic.com/en/docs/claude-code/hooks

## Relevant External Hook Surface

Claude Code exposes hook events including:

- `PreToolUse`
- `PostToolUse`
- `Notification`
- `UserPromptSubmit`
- `Stop`
- `SubagentStop`
- `PreCompact`
- `SessionStart`
- `SessionEnd`

For `way-island`, the important point is that Claude Code has a real notification event for permission prompts.

The Anthropic docs say `Notification` runs when Claude Code sends notifications, including when Claude needs permission to use a tool.

That is why Claude can map approval waiting directly from the hook stream without tmux inference.

## What `way-island` Uses

Current local mapping in [`cli.go`](/home/katsuma/src/github.com/ka2n/way-island/cli.go):

- `PreToolUse -> tool_start`
- `PostToolUse -> tool_end`
- `Notification -> waiting`
- `Stop -> idle`

Claude payloads are passed through mostly as-is by `parseClaudeHookPayload`.

Additional local metadata attached by `way-island hook`:

- `_ppid`
- `_agent_pid_ns_inode`
- `_agent_start_time`
- `_agent_tty`
- `_agent_tty_nr`
- `_hook_tty`
- `_jai_jail`
- `_hook_source = "claude"`

## Subagent Detection

Claude hook events are still the primary source for live state, but they are
not the only source used locally anymore.

When `way-island` first sees a Claude session, it may also inspect the local
Claude transcript files to discover spawned subagents for that parent session.

Observed local path layout:

- parent session: `~/.claude/projects/<encoded-cwd>/<session-id>.jsonl`
- subagent transcripts: `~/.claude/projects/<encoded-cwd>/<session-id>/subagents/agent-<agent-id>.jsonl`
- subagent metadata: `~/.claude/projects/<encoded-cwd>/<session-id>/subagents/agent-<agent-id>.meta.json`

Observed parent transcript behavior:

- the parent session JSONL contains `tool_use` entries for the `Agent` tool
- the corresponding `tool_result` entry includes `toolUseResult.agentId`

Observed subagent metadata behavior:

- `.meta.json` includes fields such as `agentType` and `description`

Practical consequence:

- Claude subagents can be derived from transcript files even if they do not
  appear as independent live sessions in the hook stream
- the UI can show `SUBAGENTS N` on the parent session and list the discovered
  subagents in the detail view
- `way-island` can also approximate subagent live state by reading the latest
  subagent transcript entries and mapping them to local states such as
  `working`, `tool_running`, and `idle`

## Current Behavior In This Repository

Claude sessions go through this pipeline:

```text
Claude hook payload
    -> parseClaudeHookPayload
    -> event mapping
    -> SessionManager
    -> optional transcript-backed subagent lookup
    -> overlayModel / UI
```

Important implementation rule:

- Claude sessions are excluded from the Codex tmux approval detector

Reason:

- Claude already emits a proper waiting signal via `Notification`
- we do not want Codex-specific heuristics mutating Claude state

## State Semantics Used Locally

`way-island` uses these internal states:

- `working`
- `tool_running`
- `waiting`
- `idle`

Claude-specific interpretation:

- `tool_start` means a tool is about to run
- `tool_end` means control returns to the model, which maps back to `working`
- `Notification` means user-visible waiting, including permission prompts
- `Stop` means the turn is complete and the session is idle

## Notes About JSON Output

The Claude hooks reference supports structured JSON responses for richer control, including `PreToolUse` permission decisions and `Stop` behavior.

`way-island` does not currently consume hook stdout for control decisions. It is only a passive observer plus metadata bridge.

## Current Local Limitations

- Claude subagent discovery currently depends on local transcript layout under
  `~/.claude/projects`
- `way-island` currently models Claude subagents as children of the parent
  session summary, not as fully independent hook-tracked sessions
- Claude integration is still primarily hook-driven for live state
- Claude subagent live state is transcript-derived and therefore best-effort,
  not an upstream-guaranteed hook signal
- richer Claude hook outputs such as allow/deny responses are not interpreted by `way-island`

## Implementation Pointers

- hook ingress: [`cli.go`](/home/katsuma/src/github.com/ka2n/way-island/cli.go)
- session state: [`internal/socket/session_manager.go`](/home/katsuma/src/github.com/ka2n/way-island/internal/socket/session_manager.go)
- Claude transcript lookup: [`internal/socket/claude_session_metadata.go`](/home/katsuma/src/github.com/ka2n/way-island/internal/socket/claude_session_metadata.go)
- UI store: [`overlay_model.go`](/home/katsuma/src/github.com/ka2n/way-island/overlay_model.go)

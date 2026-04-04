# Agent Hook References

Last verified: 2026-04-04

## Claude Code

- Anthropic Claude Code hooks reference
  - https://docs.anthropic.com/en/docs/claude-code/hooks
  - Key point used by `way-island`: `Notification` includes permission prompts.

## Codex

- Codex hooks
  - https://developers.openai.com/codex/hooks
  - Key points used by `way-island`:
    - hooks are experimental
    - hooks require `[features].codex_hooks = true`
    - `PreToolUse` and `PostToolUse` currently only expose `Bash`
    - no dedicated approval-request hook is documented in the current surface used here

- Codex config reference
  - https://developers.openai.com/codex/config-reference
  - Key point used by `way-island`: `features.codex_hooks` exists and is documented.

- Codex open source config notes
  - https://github.com/openai/codex/blob/main/docs/config.md
  - Key point used by `way-island`: project docs point readers to the OpenAI docs for latest config details.

## Related upstream issues

- https://github.com/openai/codex/issues/15311
- https://github.com/openai/codex/issues/16301
- https://github.com/openai/codex/issues/16484

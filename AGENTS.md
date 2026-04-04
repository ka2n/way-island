# AGENTS.md

## Principal

どうでもいいことは流行に従い、重要なことは標準に従い、ドメインのことは自ら設計する

## Tools

### Nix Development Shells

- When a project has `flake.nix`, use `nix develop` as needed to get the development environment.

### Missing Commands

When a command is not found, try these approaches:

- `, <cmd>` - Run command with temporary package (home-manager's comma). Example: `, cowsay hello`
- `nix-shell -p '<pkg>'` - Enter shell with package available. Example: `nix-shell -p jq`
- Example for `gcloud`: `nix-shell -p google-cloud-sdk`

### Available Tools

- `gemini` is the Google Gemini CLI. You can use it for web search. Run web search via Task Tool with `gemini -p 'WebSearch: ...'`.
- When you create a git worktree, use `git wt`. See: `git-wt --help`
- Use Codex for analysis when bug fixes fail 3+ times.
- Consult Codex for architecture design discussions.
- Request Codex for code review on large changes.
- Use Codex for existing code analysis and implementation planning.

## Document And Resource Management

- Reference materials should be saved in the `external-docs/` directory.
- For complex documentation tasks involving multiple sources or version research, use the tech-researcher agent.
- Use the `obsidian-cli` skill to save project-external knowledge and work notes.

Quick reference:

- Shallow clone: `git clone --depth 1 <REPO_URL> external-docs/<REPO_NAME>`
- Web docs: `save-url-to-doc <URL>`
- Prefer JSON Schema/OpenAPI when available.

## Agent Launch Rules

- After completing large code changes, defined as 3 or more files or 100+ lines, launch the `code-reviewer` agent.
- When changes span multiple files, launch `code-reviewer` agents in parallel.

## GitHub And CI

- Use `gh` command for all GitHub-related operations.
- PR descriptions: do not include a `Test plan` section.
- Never use `git push --force` on `main`.
- Post-push CI monitoring:
  1. Start `gh run watch $(gh run list -L 1 --json databaseId -q '.[0].databaseId') --exit-status` with `Bash(run_in_background=true)`.
  2. Continue with other work. No report is needed on CI success.
  3. On failure only: check logs, fix the issue, commit and push, then return to step 1. Maximum 3 attempts.

## Browser

- `agent-browser`: always use system `google-chrome-stable`.

## References

- `@RTK.md`

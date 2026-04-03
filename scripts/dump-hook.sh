#!/usr/bin/env bash
# Dump Claude Code hook payload for debugging
LOGFILE="${XDG_RUNTIME_DIR:-/tmp}/way-island-hook-dump.log"
HOOK_TYPE="${1:-unknown}"

{
  echo "=== $(date -Iseconds) === HOOK_TYPE=$HOOK_TYPE ==="
  echo "--- ENV ---"
  env | grep -iE '(claude|session|way.island)' | sort
  echo "--- STDIN ---"
  cat
  echo ""
  echo "=== END ==="
  echo ""
} >> "$LOGFILE"

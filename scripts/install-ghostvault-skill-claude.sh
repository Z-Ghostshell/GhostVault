#!/usr/bin/env bash
# Install the ghostvault skill for Claude Code (.claude/skills/ghostvault/SKILL.md).
# Canonical source: docs/integration/skills/ghostvault/SKILL.md
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SKILL_SRC="$REPO_ROOT/docs/integration/skills/ghostvault/SKILL.md"

usage() {
  cat <<EOF
Usage: $(basename "$0") [-g|--global] [TARGET_DIR]

  TARGET_DIR   Project root to receive .claude/skills/ (default: current directory).
  -g, --global Install to ~/.claude/skills/ (personal; all projects on this machine).
  -h, --help   Show this help.
EOF
}

GLOBAL=false
DEST=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    -g | --global) GLOBAL=true; shift ;;
    -h | --help) usage; exit 0 ;;
    -*)
      echo "unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
    *)
      if [[ -n "$DEST" ]]; then
        echo "extra argument: $1 (expected at most one TARGET_DIR)" >&2
        exit 1
      fi
      DEST="$1"
      shift
      ;;
  esac
done

if [[ ! -f "$SKILL_SRC" ]]; then
  echo "missing bundled skill: $SKILL_SRC" >&2
  exit 1
fi

if $GLOBAL; then
  OUT_DIR="${HOME}/.claude/skills/ghostvault"
else
  BASE="$(cd "${DEST:-.}" && pwd)"
  OUT_DIR="$BASE/.claude/skills/ghostvault"
fi

mkdir -p "$OUT_DIR"
install -m 0644 "$SKILL_SRC" "$OUT_DIR/SKILL.md"
echo "Installed Claude Code skill: $OUT_DIR/SKILL.md"

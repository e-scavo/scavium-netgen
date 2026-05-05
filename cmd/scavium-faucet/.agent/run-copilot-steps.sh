#!/usr/bin/env bash
set -euo pipefail

# SCAVIUM Faucet Copilot step runner
# Purpose: orchestrate sequential .agent/step*.md prompts with explicit checkpoints.
# It does not call external AI services automatically. It prepares the exact prompt,
# opens/copies it when possible, waits for the operator to run it in Copilot Chat,
# then runs validation and records the result before advancing.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
AGENT_DIR="$ROOT_DIR/cmd/scavium-faucet/.agent"
CURRENT_PROMPT="$AGENT_DIR/current-prompt.md"
LOG_DIR="$AGENT_DIR/run-logs"
DEFAULT_VALIDATE=(go test ./...)

usage() {
  cat <<'EOF'
Usage:
  cmd/scavium-faucet/.agent/run-copilot-steps.sh [options] [step-file ...]

Options:
  --from STEP        Start at a step file name, for example step20.2.0.md
  --to STEP          Stop at a step file name, for example step21.1.0.md
  --dry-run          Print selected steps without writing current-prompt.md
  --no-validate      Do not run validation after each accepted step
  --validate CMD     Validation command to run after each step. Repeatable.
                    Example: --validate 'go test ./...' --validate 'make build -B'
  -h, --help         Show this help

Default behavior:
  - Selects cmd/scavium-faucet/.agent/step*.md in lexical order.
  - Requires a clean git working tree before each step.
  - Writes cmd/scavium-faucet/.agent/current-prompt.md.
  - Pauses so the operator can paste/run the prompt in Copilot Chat.
  - Runs validation after the operator confirms the step is done.
  - Appends a log entry under cmd/scavium-faucet/.agent/run-logs/.
EOF
}

FROM_STEP=""
TO_STEP=""
DRY_RUN=0
RUN_VALIDATE=1
CUSTOM_VALIDATE=()
POSITIONAL=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --from)
      FROM_STEP="${2:-}"
      shift 2
      ;;
    --to)
      TO_STEP="${2:-}"
      shift 2
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    --no-validate)
      RUN_VALIDATE=0
      shift
      ;;
    --validate)
      CUSTOM_VALIDATE+=("${2:-}")
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      POSITIONAL+=("$1")
      shift
      ;;
  esac
done

cd "$ROOT_DIR"
mkdir -p "$LOG_DIR"

if [[ ! -f "$AGENT_DIR/rules.md" || ! -f "$AGENT_DIR/commands.md" ]]; then
  echo "ERROR: missing $AGENT_DIR/rules.md or $AGENT_DIR/commands.md" >&2
  exit 1
fi

mapfile -t ALL_STEPS < <(find "$AGENT_DIR" -maxdepth 1 -type f -name 'step*.md' -printf '%f\n' | sort)

if [[ ${#POSITIONAL[@]} -gt 0 ]]; then
  SELECTED=("${POSITIONAL[@]}")
else
  SELECTED=()
  include=0
  [[ -z "$FROM_STEP" ]] && include=1
  for step in "${ALL_STEPS[@]}"; do
    if [[ -n "$FROM_STEP" && "$step" == "$FROM_STEP" ]]; then
      include=1
    fi
    if [[ "$include" -eq 1 ]]; then
      SELECTED+=("$step")
    fi
    if [[ -n "$TO_STEP" && "$step" == "$TO_STEP" ]]; then
      break
    fi
  done
fi

if [[ ${#SELECTED[@]} -eq 0 ]]; then
  echo "ERROR: no step files selected" >&2
  exit 1
fi

for step in "${SELECTED[@]}"; do
  if [[ ! -f "$AGENT_DIR/$step" ]]; then
    echo "ERROR: missing step file: $AGENT_DIR/$step" >&2
    exit 1
  fi
done

if [[ "$DRY_RUN" -eq 1 ]]; then
  printf 'Selected steps:\n'
  printf '  %s\n' "${SELECTED[@]}"
  exit 0
fi

VALIDATE_CMDS=("${DEFAULT_VALIDATE[@]}")
if [[ ${#CUSTOM_VALIDATE[@]} -gt 0 ]]; then
  VALIDATE_CMDS=("${CUSTOM_VALIDATE[@]}")
fi

for step in "${SELECTED[@]}"; do
  echo "==> Preparing $step"

  if [[ -n "$(git status --porcelain)" ]]; then
    echo "ERROR: working tree is not clean before $step" >&2
    echo "Commit, stash, or revert changes before continuing." >&2
    exit 1
  fi

  cat > "$CURRENT_PROMPT" <<EOF
Execute cmd/scavium-faucet/.agent/$step following cmd/scavium-faucet/.agent/rules.md and cmd/scavium-faucet/.agent/commands.md.

Hard requirements:
- Treat the current repository as the only source of truth.
- Do not assume undocumented structure or behavior.
- Before editing, report:
  1. Files read
  2. Files to modify/create
  3. Minimal implementation plan
- Keep changes minimal, backward-compatible, and production-safe.
- Do not perform broad refactors.
- Do not introduce heavy dependencies.
- Update documentation incrementally if behavior, scope, or operator workflow changes.
- If something is unclear or missing in the step file, do not assume: explicitly state the uncertainty.
- After implementation, provide:
  1. Complete list of modified/created files
  2. Validation commands executed
  3. Test/build results
  4. Full git commands:
     - git checkout -b <branch>
     - git add <files>
     - git commit -m "<message>"
     - git checkout main
     - git merge <branch>
     - git branch -d <branch>
EOF

  echo "Prompt written to: $CURRENT_PROMPT"
  if command -v xclip >/dev/null 2>&1; then
    xclip -selection clipboard < "$CURRENT_PROMPT" && echo "Prompt copied to clipboard with xclip."
  elif command -v xsel >/dev/null 2>&1; then
    xsel --clipboard --input < "$CURRENT_PROMPT" && echo "Prompt copied to clipboard with xsel."
  elif command -v pbcopy >/dev/null 2>&1; then
    pbcopy < "$CURRENT_PROMPT" && echo "Prompt copied to clipboard with pbcopy."
  fi

  echo "Open $CURRENT_PROMPT, paste it into Copilot Chat, and let Copilot complete the step."
  read -r -p "Press ENTER after Copilot finished and you reviewed the changes, or Ctrl-C to abort. " _

  if [[ "$RUN_VALIDATE" -eq 1 ]]; then
    for cmd in "${VALIDATE_CMDS[@]}"; do
      echo "==> Running validation: $cmd"
      bash -lc "$cmd"
    done
  fi

  log_file="$LOG_DIR/${step%.md}-$(date -u +%Y%m%dT%H%M%SZ).log"
  {
    echo "step=$step"
    echo "timestamp_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "status=operator_confirmed"
    echo "validation_enabled=$RUN_VALIDATE"
    printf 'validation_commands=%s\n' "${VALIDATE_CMDS[*]}"
    echo "git_status_after_step:"
    git status --short
  } > "$log_file"
  echo "Log written to: $log_file"
  echo "Review, commit, and merge this step before running the next one."
  read -r -p "Press ENTER to continue to the next selected step, or Ctrl-C to stop here. " _
done

echo "All selected steps processed."

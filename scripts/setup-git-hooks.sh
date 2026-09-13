#!/usr/bin/env bash
set -euo pipefail

GIT_DIR="$(git rev-parse --git-common-dir 2>/dev/null || git rev-parse --git-dir 2>/dev/null)"
if [ -z "$GIT_DIR" ]; then
  echo "ERROR: Not inside a git repository." >&2
  exit 1
fi

HOOKS_DIR="${GIT_DIR}/hooks"
mkdir -p "$HOOKS_DIR"

PRE_PUSH_HOOK="${HOOKS_DIR}/pre-push"

cat << 'HOOK_EOF' > "$PRE_PUSH_HOOK"
#!/usr/bin/env bash
set -euo pipefail

# Only run checks if we're pushing to remote
echo "==> Running PetSpotR pre-push verification (make verify)..."
if ! make verify; then
  echo "ERROR: Pre-push verification failed. Push aborted." >&2
  echo "       Fix the issues above, or bypass with: git push --no-verify" >&2
  exit 1
fi
echo "==> Pre-push verification passed!"
HOOK_EOF

chmod +x "$PRE_PUSH_HOOK"

echo "==> Successfully installed git pre-push hook at: ${PRE_PUSH_HOOK}"
echo "    Every 'git push' will now automatically run 'make verify'."
echo "    You can bypass this in emergencies using: git push --no-verify"

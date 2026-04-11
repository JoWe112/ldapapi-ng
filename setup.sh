#!/usr/bin/env bash
# setup.sh: run once after cloning to configure local git settings
set -euo pipefail

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

ok()   { echo -e "${GREEN}[OK]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

echo "Setting up repository..."
echo ""

# --- Git hooks ---
git config core.hooksPath .githooks
ok "Git hooks activated (.githooks/)"

# --- Push / pull behaviour ---
git config push.default simple
git config pull.rebase true
git config branch.autosetuprebase always
git config fetch.prune true
ok "Push/pull settings configured"

# --- Quality of life ---
git config rerere.enabled true
git config diff.algorithm histogram
ok "rerere and histogram diff enabled"

# --- Whitespace ---
git config core.whitespace trailing-space,space-before-tab
git config apply.whitespace fix
ok "Whitespace checks enabled"

echo ""
echo -e "${GREEN}Done. Your local git config is ready.${NC}"

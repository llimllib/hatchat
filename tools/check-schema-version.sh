#!/bin/bash
#
# CI check: Ensures schema.sql changes are accompanied by a version bump.
#
# This script compares the current branch's schema against the base branch
# (default: main). If the schema has structural changes but the version
# number hasn't been incremented, it fails.
#
# Usage:
#   ./tools/check-schema-version.sh [base-branch]
#
# Examples:
#   ./tools/check-schema-version.sh          # Compare against main
#   ./tools/check-schema-version.sh develop  # Compare against develop

set -euo pipefail

BASE_BRANCH="${1:-main}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Normalize SQL by removing comments and excess whitespace
# This ensures we only detect structural changes, not comment/formatting changes
normalize_sql() {
    sed -e 's/--.*$//' -e '/^[[:space:]]*$/d' -e 's/[[:space:]]\+/ /g' -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' | \
    grep -v '^$' | \
    sort
}

# Extract schema version from the Go constant
get_go_version() {
    local file="$1"
    grep -E '^const SchemaVersion\s*=' "$file" | grep -oE '[0-9]+' || echo "0"
}

# Get the normalized schema hash
get_schema_hash() {
    local schema_file="$1"
    # Exclude the schema_version table and its INSERT from the hash
    # since those lines change when we bump the version
    cat "$schema_file" | \
        grep -v 'schema_version' | \
        normalize_sql | \
        shasum -a 256 | \
        cut -d' ' -f1
}

echo "Checking schema version consistency..."
echo "Base branch: $BASE_BRANCH"
echo ""

# Check if we can access the base branch
if ! git rev-parse --verify "$BASE_BRANCH" >/dev/null 2>&1; then
    echo -e "${YELLOW}Warning: Base branch '$BASE_BRANCH' not found. Skipping check.${NC}"
    echo "This is expected for initial commits or if the base branch hasn't been fetched."
    exit 0
fi

# Get current branch values
CURRENT_SCHEMA_HASH=$(get_schema_hash "schema.sql")
CURRENT_VERSION=$(get_go_version "server/db/version.go")

# Get base branch values
BASE_SCHEMA=$(git show "$BASE_BRANCH:schema.sql" 2>/dev/null || echo "")
BASE_VERSION_FILE=$(git show "$BASE_BRANCH:server/db/version.go" 2>/dev/null || echo "")

if [ -z "$BASE_SCHEMA" ]; then
    echo -e "${YELLOW}Warning: schema.sql not found in base branch. Skipping check.${NC}"
    exit 0
fi

if [ -z "$BASE_VERSION_FILE" ]; then
    echo -e "${YELLOW}Warning: server/db/version.go not found in base branch. Skipping check.${NC}"
    exit 0
fi

BASE_SCHEMA_HASH=$(echo "$BASE_SCHEMA" | grep -v 'schema_version' | normalize_sql | shasum -a 256 | cut -d' ' -f1)
BASE_VERSION=$(echo "$BASE_VERSION_FILE" | grep -E '^const SchemaVersion\s*=' | grep -oE '[0-9]+' || echo "0")

echo "Base branch schema hash:    $BASE_SCHEMA_HASH"
echo "Current branch schema hash: $CURRENT_SCHEMA_HASH"
echo "Base branch version:        $BASE_VERSION"
echo "Current branch version:     $CURRENT_VERSION"
echo ""

# Compare
if [ "$CURRENT_SCHEMA_HASH" = "$BASE_SCHEMA_HASH" ]; then
    echo -e "${GREEN}✓ Schema unchanged${NC}"
    exit 0
fi

# Schema changed - check if version was bumped
if [ "$CURRENT_VERSION" -gt "$BASE_VERSION" ]; then
    echo -e "${GREEN}✓ Schema changed and version bumped ($BASE_VERSION → $CURRENT_VERSION)${NC}"
    exit 0
fi

# Schema changed but version not bumped
echo -e "${RED}✗ Schema changed but version not bumped!${NC}"
echo ""
echo "The schema.sql file has structural changes, but SchemaVersion in"
echo "server/db/version.go is still $CURRENT_VERSION (same as base branch)."
echo ""
echo "To fix this:"
echo "  1. Increment SchemaVersion in server/db/version.go"
echo "  2. Update the INSERT in schema.sql to match the new version"
echo ""
echo "Example:"
echo "  const SchemaVersion = $((CURRENT_VERSION + 1))"
echo ""
echo "And in schema.sql, update:"
echo "  INSERT INTO schema_version (version)"
echo "  SELECT $((CURRENT_VERSION + 1)) WHERE NOT EXISTS (SELECT 1 FROM schema_version);"
echo ""
exit 1

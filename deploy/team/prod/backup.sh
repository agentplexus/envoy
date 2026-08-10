#!/bin/sh
# Dumps the team-mode PostgreSQL database to a timestamped, gzipped file.
#
#   ./backup.sh [output-dir]     # default: ./backups
#
# Run from deploy/team/prod/ alongside a running `docker compose up -d`
# stack. See docs/guides/team-deployment.md for restore verification.
set -eu

cd "$(dirname "$0")"

OUT_DIR="${1:-backups}"
mkdir -p "$OUT_DIR"

TIMESTAMP=$(date +%Y%m%dT%H%M%SZ)
OUT_FILE="$OUT_DIR/omniagent-team-$TIMESTAMP.sql.gz"

docker compose exec -T postgres pg_dump --clean --if-exists -U omniagent_owner omniagent_team | gzip > "$OUT_FILE"

echo "Backup written to $OUT_FILE"

#!/bin/sh
# Restores a backup produced by backup.sh into the running team-mode
# PostgreSQL database, overwriting its current contents.
#
#   ./restore.sh path/to/omniagent-team-*.sql.gz [--force]
#
# Backups are made with `pg_dump --clean --if-exists`, so the replayed dump
# drops and recreates the objects it contains -- this IS destructive to
# whatever is currently in the database. Requires --force or an interactive
# "yes" confirmation. See docs/guides/team-deployment.md for the manual
# restore-verification procedure.
set -eu

cd "$(dirname "$0")"

BACKUP_FILE="${1:?usage: restore.sh <backup-file.sql.gz> [--force]}"
FORCE="${2:-}"

if [ ! -f "$BACKUP_FILE" ]; then
  echo "backup file not found: $BACKUP_FILE" >&2
  exit 1
fi

if [ "$FORCE" != "--force" ]; then
  printf 'This overwrites the current omniagent_team database with %s. Continue? [y/N] ' "$BACKUP_FILE"
  read -r REPLY
  case "$REPLY" in
    y|Y|yes|YES) ;;
    *) echo "Aborted."; exit 1 ;;
  esac
fi

gunzip -c "$BACKUP_FILE" | docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U omniagent_owner -d omniagent_team

echo "Restore complete from $BACKUP_FILE"

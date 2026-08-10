#!/bin/sh
# Creates the non-owner application role using the password supplied via the
# APP_DB_PASSWORD compose env var. docker-entrypoint-initdb.d scripts inherit
# the postgres container's environment, so this runs with a real, per-deploy
# password rather than the dev compose's hardcoded one.
set -eu

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE ROLE omniagent_app LOGIN PASSWORD '$APP_DB_PASSWORD';
EOSQL

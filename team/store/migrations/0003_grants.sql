-- Grants for the non-owner application role ({{APP_ROLE}}).
-- The role itself is created out-of-band (compose init script or test
-- setup): roles carry credentials, which do not belong in migrations.

DO $do$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '{{APP_ROLE}}') THEN
    RAISE EXCEPTION 'application role "{{APP_ROLE}}" does not exist; create it before migrating (see deploy/team)';
  END IF;
END
$do$;

GRANT USAGE ON SCHEMA public TO {{APP_ROLE}};
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO {{APP_ROLE}};
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO {{APP_ROLE}};
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO {{APP_ROLE}};
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT USAGE, SELECT ON SEQUENCES TO {{APP_ROLE}};

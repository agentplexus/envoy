-- Create the non-owner application role for local development.
-- In a real deployment this runs in the compose init script with a real
-- password; migrations only GRANT to it (see team/store/migrations/0003).
CREATE ROLE omniagent_app LOGIN PASSWORD 'app_dev_password';

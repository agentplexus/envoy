-- Team RLS helper functions.
--
-- GUC contract (set per transaction by team/store):
--   app.current_user_id  uuid of the authenticated user ('' outside AsUser)
--   app.is_superadmin    'true' when the user is the superadmin
--   app.is_system        'true' for the auth layer / agent system context
--
-- The membership helpers are SECURITY DEFINER so policies on chats /
-- chat_members / messages can consult chat_members without RLS recursion.
-- They are owned by the migration (owner) role; the app role is a non-owner,
-- so plain ENABLE ROW LEVEL SECURITY binds fully for the app while the
-- definer functions bypass it by design.

CREATE OR REPLACE FUNCTION team_current_user_id() RETURNS uuid
LANGUAGE sql STABLE AS $fn$
  SELECT NULLIF(current_setting('app.current_user_id', true), '')::uuid
$fn$;

CREATE OR REPLACE FUNCTION team_is_superadmin() RETURNS boolean
LANGUAGE sql STABLE AS $fn$
  SELECT COALESCE(current_setting('app.is_superadmin', true), 'false') = 'true'
$fn$;

CREATE OR REPLACE FUNCTION team_is_system() RETURNS boolean
LANGUAGE sql STABLE AS $fn$
  SELECT COALESCE(current_setting('app.is_system', true), 'false') = 'true'
$fn$;

CREATE OR REPLACE FUNCTION team_is_chat_member(p_chat uuid) RETURNS boolean
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $fn$
  SELECT EXISTS (
    SELECT 1 FROM chat_members
    WHERE chat_id = p_chat AND user_id = team_current_user_id()
  )
$fn$;

CREATE OR REPLACE FUNCTION team_is_chat_owner(p_chat uuid) RETURNS boolean
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $fn$
  SELECT EXISTS (
    SELECT 1 FROM chat_members
    WHERE chat_id = p_chat AND user_id = team_current_user_id() AND role = 'owner'
  )
$fn$;

CREATE OR REPLACE FUNCTION team_is_chat_creator(p_chat uuid) RETURNS boolean
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $fn$
  SELECT EXISTS (
    SELECT 1 FROM chats
    WHERE id = p_chat AND created_by = team_current_user_id()
  )
$fn$;

-- Agent (INIT-OMNIAGENT-005) membership helpers. "Editor" = owner or
-- maintainer, both of whom may configure the agent (skills, persona,
-- secrets, visibility); only an owner may manage maintainers, so a separate
-- team_is_agent_owner helper exists for that narrower check.

CREATE OR REPLACE FUNCTION team_is_agent_editor(p_agent uuid) RETURNS boolean
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $fn$
  SELECT EXISTS (
    SELECT 1 FROM agent_roles
    WHERE agent_id = p_agent AND user_id = team_current_user_id()
  )
$fn$;

CREATE OR REPLACE FUNCTION team_is_agent_owner(p_agent uuid) RETURNS boolean
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $fn$
  SELECT EXISTS (
    SELECT 1 FROM agent_roles
    WHERE agent_id = p_agent AND user_id = team_current_user_id() AND role = 'owner'
  )
$fn$;

-- Bootstrap: a freshly created agent has no agent_roles row yet, so the
-- creator's first self-insert into agent_roles (as owner) is authorized by
-- this check instead, mirroring team_is_chat_creator.
CREATE OR REPLACE FUNCTION team_is_agent_creator(p_agent uuid) RETURNS boolean
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $fn$
  SELECT EXISTS (
    SELECT 1 FROM agents
    WHERE id = p_agent AND created_by = team_current_user_id()
  )
$fn$;

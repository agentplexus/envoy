-- Team row-level security policies. Idempotent: reapplied on every migrate.
--
-- The app connects as a NON-OWNER role, so ENABLE ROW LEVEL SECURITY binds
-- every policy for it; the owner (migration role) retains bypass, which the
-- SECURITY DEFINER helpers in 0001 rely on. Tables with no policy for an
-- operation deny that operation (messages are immutable: no UPDATE/DELETE).

-- users -----------------------------------------------------------------
ALTER TABLE users ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS users_select ON users;
CREATE POLICY users_select ON users FOR SELECT
  USING (id = team_current_user_id() OR team_is_superadmin() OR team_is_system());

DROP POLICY IF EXISTS users_insert ON users;
CREATE POLICY users_insert ON users FOR INSERT
  WITH CHECK (team_is_system() OR team_is_superadmin());

DROP POLICY IF EXISTS users_update ON users;
CREATE POLICY users_update ON users FOR UPDATE
  USING (id = team_current_user_id() OR team_is_superadmin() OR team_is_system())
  WITH CHECK (id = team_current_user_id() OR team_is_superadmin() OR team_is_system());

DROP POLICY IF EXISTS users_delete ON users;
CREATE POLICY users_delete ON users FOR DELETE
  USING (team_is_superadmin());

-- identities ------------------------------------------------------------
ALTER TABLE identities ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS identities_select ON identities;
CREATE POLICY identities_select ON identities FOR SELECT
  USING (user_id = team_current_user_id() OR team_is_superadmin() OR team_is_system());

DROP POLICY IF EXISTS identities_insert ON identities;
CREATE POLICY identities_insert ON identities FOR INSERT
  WITH CHECK (team_is_system());

DROP POLICY IF EXISTS identities_delete ON identities;
CREATE POLICY identities_delete ON identities FOR DELETE
  USING (user_id = team_current_user_id() OR team_is_superadmin());

-- allowlist -------------------------------------------------------------
ALTER TABLE allowlist ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS allowlist_select ON allowlist;
CREATE POLICY allowlist_select ON allowlist FOR SELECT
  USING (team_is_superadmin() OR team_is_system());

DROP POLICY IF EXISTS allowlist_insert ON allowlist;
CREATE POLICY allowlist_insert ON allowlist FOR INSERT
  WITH CHECK (team_is_superadmin());

DROP POLICY IF EXISTS allowlist_update ON allowlist;
CREATE POLICY allowlist_update ON allowlist FOR UPDATE
  USING (team_is_superadmin())
  WITH CHECK (team_is_superadmin());

DROP POLICY IF EXISTS allowlist_delete ON allowlist;
CREATE POLICY allowlist_delete ON allowlist FOR DELETE
  USING (team_is_superadmin());

-- magic_link_tokens: system (auth layer) only ---------------------------
ALTER TABLE magic_link_tokens ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS magic_link_tokens_all ON magic_link_tokens;
CREATE POLICY magic_link_tokens_all ON magic_link_tokens FOR ALL
  USING (team_is_system())
  WITH CHECK (team_is_system());

-- auth_sessions ----------------------------------------------------------
ALTER TABLE auth_sessions ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS auth_sessions_select ON auth_sessions;
CREATE POLICY auth_sessions_select ON auth_sessions FOR SELECT
  USING (user_id = team_current_user_id() OR team_is_superadmin() OR team_is_system());

DROP POLICY IF EXISTS auth_sessions_insert ON auth_sessions;
CREATE POLICY auth_sessions_insert ON auth_sessions FOR INSERT
  WITH CHECK (team_is_system());

DROP POLICY IF EXISTS auth_sessions_update ON auth_sessions;
CREATE POLICY auth_sessions_update ON auth_sessions FOR UPDATE
  USING (team_is_system())
  WITH CHECK (team_is_system());

DROP POLICY IF EXISTS auth_sessions_delete ON auth_sessions;
CREATE POLICY auth_sessions_delete ON auth_sessions FOR DELETE
  USING (user_id = team_current_user_id() OR team_is_superadmin() OR team_is_system());

-- chats ------------------------------------------------------------------
-- Note: superadmin is deliberately NOT content-privileged (PRD): no blanket
-- superadmin SELECT on chats/messages — administration works via
-- chat_members, not content access.
ALTER TABLE chats ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS chats_select ON chats;
CREATE POLICY chats_select ON chats FOR SELECT
  USING (team_is_chat_member(id) OR team_is_system());

DROP POLICY IF EXISTS chats_insert ON chats;
CREATE POLICY chats_insert ON chats FOR INSERT
  WITH CHECK (created_by = team_current_user_id() OR team_is_system());

DROP POLICY IF EXISTS chats_update ON chats;
CREATE POLICY chats_update ON chats FOR UPDATE
  USING (team_is_chat_owner(id) OR team_is_superadmin())
  WITH CHECK (team_is_chat_owner(id) OR team_is_superadmin());

DROP POLICY IF EXISTS chats_delete ON chats;
CREATE POLICY chats_delete ON chats FOR DELETE
  USING (team_is_chat_owner(id) OR team_is_superadmin());

-- chat_members ------------------------------------------------------------
ALTER TABLE chat_members ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS chat_members_select ON chat_members;
CREATE POLICY chat_members_select ON chat_members FOR SELECT
  USING (team_is_chat_member(chat_id) OR team_is_superadmin() OR team_is_system());

-- Creator may insert their own (owner) membership when creating a chat;
-- afterwards only owners/superadmin/system may add members.
DROP POLICY IF EXISTS chat_members_insert ON chat_members;
CREATE POLICY chat_members_insert ON chat_members FOR INSERT
  WITH CHECK (
    team_is_chat_owner(chat_id)
    OR team_is_superadmin()
    OR team_is_system()
    OR (user_id = team_current_user_id() AND team_is_chat_creator(chat_id))
  );

DROP POLICY IF EXISTS chat_members_delete ON chat_members;
CREATE POLICY chat_members_delete ON chat_members FOR DELETE
  USING (
    user_id = team_current_user_id()      -- self-leave
    OR team_is_chat_owner(chat_id)
    OR team_is_superadmin()
    OR team_is_system()
  );

-- messages ----------------------------------------------------------------
-- Immutable at v1: no UPDATE/DELETE policies exist, so both are denied.
ALTER TABLE messages ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS messages_select ON messages;
CREATE POLICY messages_select ON messages FOR SELECT
  USING (team_is_chat_member(chat_id) OR team_is_system());

DROP POLICY IF EXISTS messages_insert ON messages;
CREATE POLICY messages_insert ON messages FOR INSERT
  WITH CHECK (
    (author_type = 'user'
       AND author_user_id = team_current_user_id()
       AND team_is_chat_member(chat_id))
    OR (author_type = 'agent' AND team_is_system())
  );

-- agents (INIT-OMNIAGENT-005) ---------------------------------------------
-- Superadmin may administer (update roles/registry indirectly via the
-- service, and update this row for reassignment) but this is row-level
-- security, not column-level: the "only superadmin sets featured" rule (TRD
-- section 3, section 9 Q1) is enforced by the service layer, not by a
-- separate policy on this table.
ALTER TABLE agents ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS agents_select ON agents;
CREATE POLICY agents_select ON agents FOR SELECT
  USING (
    team_is_agent_editor(id)
    OR visibility = 'listed'
    OR team_is_superadmin()
    OR team_is_system()
  );

DROP POLICY IF EXISTS agents_insert ON agents;
CREATE POLICY agents_insert ON agents FOR INSERT
  WITH CHECK (created_by = team_current_user_id() OR team_is_system());

DROP POLICY IF EXISTS agents_update ON agents;
CREATE POLICY agents_update ON agents FOR UPDATE
  USING (team_is_agent_editor(id) OR team_is_superadmin() OR team_is_system())
  WITH CHECK (team_is_agent_editor(id) OR team_is_superadmin() OR team_is_system());

DROP POLICY IF EXISTS agents_delete ON agents;
CREATE POLICY agents_delete ON agents FOR DELETE
  USING (team_is_agent_owner(id) OR team_is_superadmin());

-- agent_skills --------------------------------------------------------------
ALTER TABLE agent_skills ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS agent_skills_select ON agent_skills;
CREATE POLICY agent_skills_select ON agent_skills FOR SELECT
  USING (
    team_is_agent_editor(agent_id)
    OR EXISTS (SELECT 1 FROM agents WHERE id = agent_id AND visibility = 'listed')
    OR team_is_superadmin()
    OR team_is_system()
  );

DROP POLICY IF EXISTS agent_skills_insert ON agent_skills;
CREATE POLICY agent_skills_insert ON agent_skills FOR INSERT
  WITH CHECK (team_is_agent_editor(agent_id) OR team_is_superadmin() OR team_is_system());

DROP POLICY IF EXISTS agent_skills_delete ON agent_skills;
CREATE POLICY agent_skills_delete ON agent_skills FOR DELETE
  USING (team_is_agent_editor(agent_id) OR team_is_superadmin() OR team_is_system());

-- agent_roles ---------------------------------------------------------------
-- Owner manages maintainers; a user may remove their own role (self-leave);
-- the creator's first self-insert (as owner) is authorized via
-- team_is_agent_creator before any agent_roles row exists for that agent.
ALTER TABLE agent_roles ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS agent_roles_select ON agent_roles;
CREATE POLICY agent_roles_select ON agent_roles FOR SELECT
  USING (team_is_agent_editor(agent_id) OR team_is_superadmin() OR team_is_system());

DROP POLICY IF EXISTS agent_roles_insert ON agent_roles;
CREATE POLICY agent_roles_insert ON agent_roles FOR INSERT
  WITH CHECK (
    team_is_agent_owner(agent_id)
    OR team_is_superadmin()
    OR team_is_system()
    OR (user_id = team_current_user_id() AND role = 'owner' AND team_is_agent_creator(agent_id))
  );

DROP POLICY IF EXISTS agent_roles_delete ON agent_roles;
CREATE POLICY agent_roles_delete ON agent_roles FOR DELETE
  USING (
    user_id = team_current_user_id()      -- self-leave
    OR team_is_agent_owner(agent_id)
    OR team_is_superadmin()
    OR team_is_system()
  );

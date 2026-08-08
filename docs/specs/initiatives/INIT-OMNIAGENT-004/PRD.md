# OmniAgent Skill Secrets — Product Requirements Document

> **Initiative:** `INIT-OMNIAGENT-004`
> **Status:** Draft
> **Date:** 2026-08-04

> **Rescope note (2026-08-04):** Secrets are **agent-scoped**, not per-end-user.
> Following `INIT-OMNIAGENT-005` (agent as a first-class entity), a secret belongs
> to an **agent** and is managed by that agent's **owner/maintainers** (not every
> chat participant); every chat with the agent uses the agent's secrets. This
> matches the GitHub repo-secrets model and the product decision of 2026-08-04.
> The Phase-1 core (declaration, config bindings, injection) is unchanged and
> single-operator. Phase 2's store keys on `agent_id` (managed via the agent's
> `Can(CapConfigure)`), Phase 3 resolves per **agent** (not per acting user), and
> Phase 4's UI is the owner/maintainer surface. Per-user secrets are a possible
> future extension, not v1.

## Problem Statement

Skills routinely need credentials — a GitHub token, an OpenAI key, an SMTP
password. Today there is no path for a skill to *declare* the secrets it needs,
and no managed way to *supply* them: MCP skills get an `Env` map hand-wired in
Go, the config file has a fixed list of credential fields, and there is no
per-user story at all. In a shared (team/family) agent, each member needs their
*own* credentials — my GitHub token, not yours — but nothing scopes secrets to a
user.

## Vision

Bring the **GitHub Actions secrets model** to OmniAgent skills:

- A skill **declares** the secrets it needs by logical name and the env var it
  expects (like `secrets.GITHUB_TOKEN`).
- Secrets are stored in a managed **vault** — omnivault for operator/global
  secrets, and an encrypted, RLS-scoped store for **per-user** secrets.
- The agent **injects** only a skill's declared secrets, resolved for the
  acting user, and never exposes values in prompts or logs.
- In team mode, each member manages **their own** secrets in the web UI,
  entering the env-var-named values the skills ask for — exactly like adding a
  repository secret on GitHub.

## Users

| Role | Need |
|------|------|
| **Skill author** | Declare required/optional secrets once, in `SKILL.md`, without touching agent code. |
| **Operator** (single or team) | Bind global secrets (team-wide defaults, service accounts) to vault references, once, in config. |
| **Member** (team mode) | Add/update/remove *their own* secrets through the UI; see which skills need which secrets. |

## User Stories

### Declaration & operator binding

- **US-1**: As a skill author, I declare in `SKILL.md` that my skill needs
  `GITHUB_TOKEN` (required) and `GITHUB_ENTERPRISE_URL` (optional), including the
  env var name each is injected as.
- **US-2**: As an operator, I bind a declared secret to a vault reference in
  config (`GITHUB_TOKEN: op://Work/GitHub/token`) so it resolves at startup — a
  single global value for the whole deployment.
- **US-3**: As an operator, when a **required** secret is unbound, the skill is
  disabled at load with a clear message telling me exactly what to set — no
  silent half-working skills.

### Per-user secrets (team mode)

- **US-4**: As a member, I open Settings, see that the `github` skill needs a
  secret named `GITHUB_TOKEN`, paste my token, and save it — no config file, no
  restart.
- **US-5**: As a member, when I use a skill in my chat, it runs with **my**
  secrets; another member's chat uses theirs. My secrets are invisible to every
  other member (including, at the app layer, the superadmin).
- **US-6**: As a member, I only ever see my secret **names**, never their values
  after saving (write-only, like GitHub); I can overwrite or delete them.
- **US-7**: As a member, if a skill needs a secret I haven't set, the agent tells
  me which secret to add rather than failing opaquely.

### Safety

- **US-8**: As anyone, secret values never appear in the model's prompt, the
  chat transcript, or the server logs.
- **US-9**: As an operator, per-user secrets are encrypted at rest, so a database
  backup does not leak them.

## Requirements

### Must Have

1. **Declaration** — a `secrets` block in `SKILL.md` frontmatter: name,
   description, required flag, and the env var name to inject as.
2. **Global binding** — a config `secrets:` map and per-skill overrides,
   resolving vault references (`op://`, `bw://`, `file://`, `env://`,
   `aws-sm://`, …) through omnivault, with plain values passing through.
3. **Injection** — a skill receives only its declared secrets: MCP skills via
   `Env`, compiled skills via a `SecretsAware` interface, OpenAPI skills via
   their auth config.
4. **Required-secret gating** — a skill with an unresolved required secret is
   marked unavailable, with an actionable message.
5. **Redaction** — resolved secret values are masked in logs and never enter
   the prompt/transcript.
6. **Per-user store (team mode)** — an encrypted, RLS-scoped `user_secrets`
   table; values write-only from the UI; per-user isolation enforced by RLS and
   the app layer.
7. **Per-user resolution precedence** — for a skill invoked in a member's chat:
   the member's own secret wins; else the global binding; else unbound.
8. **Secrets UI (team mode)** — a member manages their own secrets by env-var
   name; the UI surfaces which skills declare which secrets and whether each is
   set.

### Should Have

- Admin view of global bindings and which are set (values never shown).
- An audit line when a per-user secret is created/updated/deleted (no value).

### Non-Goals

- A general-purpose secrets manager UI beyond what skills declare (we are not
  rebuilding 1Password).
- Sharing one member's secret with another member.
- Per-secret fine-grained ACL beyond owner-only (member) / operator (global).
- Secret rotation automation (manual re-entry / re-bind at v1).
- End-to-end encryption where the server cannot read values (the server must
  decrypt to inject; encryption is at-rest against DB compromise).

## Success Criteria

- A skill author adds `secrets:` to `SKILL.md`; an operator binds one value in
  config; the skill runs — no agent code change.
- In team mode, a member pastes a token in the UI and immediately uses the skill
  in chat with that token; a second member does the same independently and
  neither sees the other's value.
- A required-but-unset secret disables the skill (single-operator) or prompts the
  member to add it (team), never failing silently.
- No secret value appears in logs, prompt, or transcript in any flow.

## Relationship to Other Initiatives

- **Depends on `INIT-OMNIAGENT-003`** for the per-user phases: the RLS store
  (Phase 1), acting-user/chat context (Phase 3), and web UI (Phase 4).
- The **core** phase (declaration + global binding + injection) is independent
  and benefits single-operator deployments with no team dependency.

# Virtual Agents

In team mode, OmniAgent hosts **virtual agents**: named personas, each bound to
a chosen subset of the deployment's skills and its own agent-scoped secrets. A
virtual agent is what people discover in a catalog and chat with; who may
*configure* it is controlled by per-agent roles, entirely separate from who may
*converse* with it.

This guide covers the web surfaces added for virtual agents — the **My Agents**
configuration area, the **Catalog**, and the superadmin **Curation** view — plus
the HTTP API behind them.

!!! note "Team mode only"
    These surfaces appear only when the deployment runs in team mode
    (`team.enabled: true`), which implies authentication. In personal mode there
    is a single implicit agent and no catalog, so the tabs are not shown. See
    [Web UI Authentication](authentication.md) for how sign-in works.

## The tabs

When signed in to a team deployment the top navigation shows:

- **Chats** — your DMs and group chats (unchanged).
- **Catalog** — browse featured and listed agents and start a chat with one.
- **My Agents** — create and configure the agents you own or maintain.
- **Curation** — *superadmin only*: promote agents to Featured.

## Roles and capabilities

Each agent has its own roles, independent of chat membership:

| Role | May do |
|------|--------|
| **Owner** | Everything: edit config, set skills, manage maintainers, set visibility, delete. |
| **Maintainer** | Edit config, set skills, set visibility. **Not** manage maintainers or delete. |
| **Superadmin** | Administer any agent (including the above) and curate the Featured list. Superadmin does not automatically receive an agent's secret values. |
| **Any user** | Start a chat with an agent that is **Listed** (or one they can already see). |

Conversing with an agent never grants any configuration right. The web UI only
renders controls you are permitted to use, and the server re-checks every
mutation regardless.

## My Agents

The **My Agents** tab lists the agents you own or maintain.

### Create an agent

Click **New agent** and fill in:

- **Slug** — a unique handle, 3–32 characters of `a-z`, `0-9`, `-` or `_`,
  starting with a letter or digit (e.g. `research-bot`). Used to @-mention the
  agent in group chats.
- **Name** — the display name shown in the catalog and chats.
- **Description** — a short blurb shown to people browsing the catalog.
- **Persona** — the system prompt that gives the agent its behavior.
- **Model** / **Provider** — optional; leave blank to inherit the deployment
  defaults.

You become the agent's **owner** automatically. New agents start **Private**.

### Configure an agent

Open an agent from the list to edit it in panels:

- **Configuration** — name, description, persona, model, provider.
- **Skills** — tick the deployment skills this agent may use. Only skills in the
  deployment's available-skills catalog (minus the operator deny-list) can be
  enabled; an unknown or blocked skill is rejected and nothing is changed.
- **Visibility** — Private or Listed (see below). Owners and maintainers only.
- **Maintainers** — add a co-editor by username, or remove one. Owners only.
  Anyone may **Leave** an agent they hold a role on; the sole owner cannot leave
  while other role holders remain (a superadmin reassigns ownership).
- **Danger zone** — **Delete agent** (owner/superadmin) removes the agent and,
  by cascade, its skills and roles.

## Catalog

The **Catalog** tab is the discovery surface. It has two sections:

- **Featured** — agents a superadmin has promoted.
- **All agents** — agents whose owners set them to Listed.

Each card offers **Chat** (start or reopen a one-to-one DM with the agent) and
**New group** (create a group chat bound to the agent; @-mention its slug to get
a reply). Starting a chat drops you into the **Chats** tab with the new
conversation open.

Private agents never appear in the catalog for people who cannot already see
them, and the **Chat** actions are shown only when you are allowed to start one.

### Visibility

| Visibility | Who can discover it | Who can start a chat |
|------------|---------------------|----------------------|
| **Private** (default) | Owners, maintainers, superadmin | Owners, maintainers, superadmin |
| **Listed** | Everyone in the deployment | Everyone in the deployment |

Owners and maintainers change visibility; **Featured** is separate and
superadmin-only.

## Curation (superadmin)

The **Curation** tab lists Featured and Listed agents with a **Feature** /
**Unfeature** toggle. Featuring promotes a Listed agent to the top of everyone's
catalog. Featuring is deliberately kept distinct from visibility: owners opt
their agent into being Listed; a superadmin decides what the whole deployment
sees first.

## End-to-end walkthrough

The full lifecycle, and a quick way to verify a deployment:

1. **Create** — as any allowlisted user, open **My Agents → New agent**, give it
   a slug, name, and persona, and create it. You are its owner; it starts
   Private.
2. **Configure** — open the agent, tick the skills it may use under **Skills**,
   and save. Optionally add a co-editor under **Maintainers**.
3. **List** — under **Visibility**, set it to **Listed** so the deployment can
   discover it.
4. **Feature** *(superadmin)* — in the **Curation** tab, click **Feature** to
   promote it to the top of everyone's catalog.
5. **Discover** — as a different, non-editor user, open **Catalog**; the agent
   appears (under Featured if promoted, otherwise All agents) with a **Chat**
   action.
6. **Chat** — click **Chat** to start a DM, or **New group** and @-mention the
   agent's slug; the reply runs on that agent's persona + skills. A plain
   conversant sees no configuration surface at any point.

## HTTP API

The web UI is a thin client over these endpoints. All require an authenticated
session cookie; mutations also require the `X-OmniAgent-CSRF` header. Authorization
is enforced per-agent server-side.

| Method & path | Purpose |
|---------------|---------|
| `GET /api/catalog` | Featured + Listed agents visible to the caller, each with `canStart`. |
| `GET /api/agents` | Agents the caller owns or maintains. |
| `POST /api/agents` | Create an agent (caller becomes owner). |
| `GET /api/agents/{id}` | Agent detail: config, enabled + available skills, and the caller's capabilities. |
| `PATCH /api/agents/{id}` | Update name/description/persona/model/provider. |
| `DELETE /api/agents/{id}` | Delete the agent (owner/superadmin). |
| `PUT /api/agents/{id}/skills` | Replace the enabled-skill set. |
| `GET /api/agents/{id}/roles` | List role holders (owners + maintainers). |
| `POST /api/agents/{id}/maintainers` | Add a maintainer by username (owner/superadmin). |
| `DELETE /api/agents/{id}/maintainers/{userId}` | Remove a maintainer (owner/superadmin). |
| `POST /api/agents/{id}/leave` | Remove your own role. |
| `PUT /api/agents/{id}/visibility` | Set `private` or `listed` (owner/maintainer). |
| `PUT /api/agents/{id}/featured` | Set featured (superadmin only). |
| `POST /api/chats/agents/{agentId}/dm` | Start (or reopen) a DM with the agent. |
| `POST /api/chats/agents/{agentId}/group` | Create a group chat bound to the agent. |

## Secrets

Agent-scoped secrets (per-agent environment injected into an agent's skills,
including its MCP subprocesses) are configured out-of-band, not through this UI —
managing secret **values** is intentionally kept off the web surface. See the
deployment configuration for `team.secrets`.

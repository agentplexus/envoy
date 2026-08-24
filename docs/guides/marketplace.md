# Agent Marketplace

OmniAgent owns the reusable marketplace model for agent and skill discovery.
Applications such as UIForge should integrate this package instead of creating
their own agent marketplace concepts.

The marketplace package is intentionally storage-agnostic. It defines portable
listings, filters, and a provider interface; host applications decide whether
listings come from static config, team-mode storage, SpiceDB-backed policy, or a
remote registry.

## Core Concepts

- **Agent listings** describe persona-style agents that an app can offer to a
  user. They include provider/model metadata, skills, tools, capabilities,
  visibility, and featured status.
- **Skill listings** describe reusable capabilities that can be attached to
  agents. Secret declarations are exposed by name only; secret values are never
  part of marketplace payloads.
- **Capabilities** are host-owned strings such as `uiforge.query.run` or
  `omniroadmap.roadmap.read`. OmniAgent does not need to understand every
  application's domain to filter and present marketplace entries.
- **Tools** are executable actions. A tool may reference the capability that
  authorizes or explains it.
- **Providers** expose the catalog. The built-in static provider is useful for
  embedded catalogs and tests; database-backed providers can satisfy the same
  interface.

## Go Usage

```go
provider := marketplace.NewStaticProvider([]marketplace.AgentListing{{
    ID:         "analytics-assistant",
    Name:       "Analytics Assistant",
    Visibility: marketplace.VisibilityListed,
    Featured:   true,
    Enabled:    true,
    Capabilities: []marketplace.CapabilityRef{{
        Name: "uiforge.query.run",
    }},
}}, nil)

catalog, err := provider.Catalog(ctx, marketplace.Filter{
    Query:        "analytics",
    Capabilities: []string{"uiforge.query.run"},
})
```

## UIForge Integration Shape

UIForge should depend on OmniAgent's marketplace package and provide UIForge
capability names when it publishes agents or skills:

- `uiforge.question.read`
- `uiforge.question.write`
- `uiforge.query.run`
- `uiforge.query.explain`
- `uiforge.field_values.read`

This keeps the app-specific authorization boundary in UIForge while allowing
the same marketplace UI and provider contracts to be reused in OmniRoadmap,
VisionStudio, or other products.

## Team Mode

Team mode already has a user-scoped catalog for virtual agents. That catalog can
be adapted to the marketplace provider interface without changing its existing
authorization model. The service layer should remain the source of truth for
visibility, role checks, and start-chat authorization.


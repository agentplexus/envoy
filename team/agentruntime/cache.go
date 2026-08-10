// Package agentruntime provides per-agent runtime instances for team mode
// (RMI-OMNIAGENT-309). A chat names an agent; each turn must run on an instance
// built from that agent's persona + enabled skills (and, once RMI-310 lands, its
// agent-scoped secrets). Building an instance is expensive — it opens an LLM
// client and loads skills — so instances are built lazily on first use and held
// in a bounded LRU cache: a busy deployment keeps only its hottest agents
// resident, and an idle agent is evicted rather than pinned in memory forever.
//
// Cache satisfies the chats.AgentRuntime seam (Slug + Processor), so the chats
// service routes an agent-bound chat's turns to the agent's own instance. The
// cache itself depends only on two seams — a ConfigLoader (reads an agent's
// runtime configuration by ID, in system context) and a Builder (turns that
// configuration into a processor) — so it is independent of the LLM/skill stack
// and unit-testable with fakes. AgentBuilder (builder.go) is the production
// Builder that constructs a real *agent.Agent.
package agentruntime

import (
	"container/list"
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/team/chats"
)

// DefaultMaxInstances bounds how many per-agent instances the cache holds
// resident before evicting the least-recently-used one. TRD §9 Q3 leaves the
// eviction policy to be tuned in Phase 4; this is a conservative default.
const DefaultMaxInstances = 64

// AgentConfig is an agent's resolved runtime configuration, loaded by ID. The
// runtime is a system principal (not a user), so a ConfigLoader reads it in
// system context, independent of RLS visibility.
type AgentConfig struct {
	ID       uuid.UUID
	Slug     string
	Name     string
	Persona  string
	Model    string
	Provider string
	Skills   []string
}

// ConfigLoader reads an agent's runtime configuration by ID, in system context.
// The two methods are split so a group turn can make the cheap @-mention check
// (AgentSlug) without triggering a full config load or instance build.
type ConfigLoader interface {
	// AgentSlug returns just the agent's slug — the cheap read a group turn
	// makes for @-mention matching, without building the instance.
	AgentSlug(ctx context.Context, agentID uuid.UUID) (string, error)
	// LoadConfig returns the agent's full runtime configuration.
	LoadConfig(ctx context.Context, agentID uuid.UUID) (AgentConfig, error)
}

// Builder turns a resolved AgentConfig into a processor. It is the injection
// point for the LLM/skill stack (and, in RMI-310, agent-scoped secret binding),
// keeping the cache independent of the agent package and unit-testable. A
// processor that holds resources (e.g. *agent.Agent) should implement io.Closer;
// the cache closes it on eviction and on Close.
type Builder interface {
	Build(ctx context.Context, cfg AgentConfig) (chats.AgentProcessor, error)
}

// Config configures the cache.
type Config struct {
	// Loader reads agents' runtime configuration. Required.
	Loader ConfigLoader
	// Builder builds an instance from a config. Required.
	Builder Builder
	// MaxInstances bounds the resident instance count; <=0 uses
	// DefaultMaxInstances.
	MaxInstances int
	// Logger defaults to slog.Default().
	Logger *slog.Logger
}

// Cache satisfies the chats.AgentRuntime seam.
var _ chats.AgentRuntime = (*Cache)(nil)

// Cache is a lazy, bounded-LRU cache of per-agent runtime instances. It
// satisfies chats.AgentRuntime. Safe for concurrent use.
type Cache struct {
	loader  ConfigLoader
	builder Builder
	max     int
	logger  *slog.Logger

	mu    sync.Mutex
	ll    *list.List                  // front = most-recently-used *entry
	items map[uuid.UUID]*list.Element // agentID -> element holding *entry
}

// entry holds one agent's instance. once serializes the (single) build so
// concurrent callers for the same agent share one instance rather than racing
// to build duplicates; the build runs outside the cache mutex so building one
// agent never blocks turns for another.
type entry struct {
	id   uuid.UUID
	once sync.Once
	proc chats.AgentProcessor
	err  error
}

// New creates a runtime cache.
func New(cfg Config) (*Cache, error) {
	if cfg.Loader == nil {
		return nil, fmt.Errorf("agentruntime: loader is required")
	}
	if cfg.Builder == nil {
		return nil, fmt.Errorf("agentruntime: builder is required")
	}
	if cfg.MaxInstances <= 0 {
		cfg.MaxInstances = DefaultMaxInstances
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Cache{
		loader:  cfg.Loader,
		builder: cfg.Builder,
		max:     cfg.MaxInstances,
		logger:  cfg.Logger,
		ll:      list.New(),
		items:   make(map[uuid.UUID]*list.Element),
	}, nil
}

// Slug returns the bound agent's slug for @-mention matching. It does not build
// or cache an instance — only the cheap config read.
func (c *Cache) Slug(ctx context.Context, agentID uuid.UUID) (string, error) {
	return c.loader.AgentSlug(ctx, agentID)
}

// Processor returns the agent's instance, building it on first use and caching
// it (evicting the least-recently-used instance when over capacity). Concurrent
// callers for the same agent share a single build. A build error is not cached:
// the poisoned entry is dropped so the next call retries.
func (c *Cache) Processor(ctx context.Context, agentID uuid.UUID) (chats.AgentProcessor, error) {
	c.mu.Lock()
	if el, ok := c.items[agentID]; ok {
		c.ll.MoveToFront(el)
		e := el.Value.(*entry)
		c.mu.Unlock()
		return c.build(ctx, e)
	}
	e := &entry{id: agentID}
	c.items[agentID] = c.ll.PushFront(e)
	evicted := c.evictLocked()
	c.mu.Unlock()

	c.closeAll(evicted)
	return c.build(ctx, e)
}

// build runs (or awaits) the one-time build for an entry and returns its result.
// A failed build drops the entry from the cache so it is retried next time.
func (c *Cache) build(ctx context.Context, e *entry) (chats.AgentProcessor, error) {
	e.once.Do(func() {
		cfg, err := c.loader.LoadConfig(ctx, e.id)
		if err != nil {
			e.err = fmt.Errorf("load agent config: %w", err)
			return
		}
		proc, err := c.builder.Build(ctx, cfg)
		if err != nil {
			e.err = fmt.Errorf("build agent runtime: %w", err)
			return
		}
		e.proc = proc
	})
	if e.err != nil {
		c.drop(e)
		return nil, e.err
	}
	return e.proc, nil
}

// evictLocked removes least-recently-used entries until the cache is within
// capacity, returning the evicted processors for the caller to close outside
// the mutex. Must hold c.mu.
func (c *Cache) evictLocked() []chats.AgentProcessor {
	var evicted []chats.AgentProcessor
	for len(c.items) > c.max {
		back := c.ll.Back()
		if back == nil {
			break
		}
		e := c.ll.Remove(back).(*entry)
		delete(c.items, e.id)
		if e.proc != nil {
			evicted = append(evicted, e.proc)
		}
	}
	return evicted
}

// drop removes a specific entry from the cache if it is still the mapped one
// (a newer entry for the same agent may have replaced it). It does not close the
// processor — drop is used for poisoned (failed-build) entries, whose proc is nil.
func (c *Cache) drop(e *entry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[e.id]; ok && el.Value.(*entry) == e {
		c.ll.Remove(el)
		delete(c.items, e.id)
	}
}

// Invalidate evicts an agent's cached instance (if any) and closes it, so the
// next turn rebuilds from current configuration. Call it when an agent's
// persona/skills change. It is a no-op when the agent is not resident.
func (c *Cache) Invalidate(agentID uuid.UUID) {
	c.mu.Lock()
	el, ok := c.items[agentID]
	if !ok {
		c.mu.Unlock()
		return
	}
	e := c.ll.Remove(el).(*entry)
	delete(c.items, agentID)
	c.mu.Unlock()
	if e.proc != nil {
		c.closeAll([]chats.AgentProcessor{e.proc})
	}
}

// Close evicts and closes every resident instance. The cache is reusable
// afterward (a later Processor call rebuilds).
func (c *Cache) Close() error {
	c.mu.Lock()
	procs := make([]chats.AgentProcessor, 0, len(c.items))
	for _, el := range c.items {
		if e := el.Value.(*entry); e.proc != nil {
			procs = append(procs, e.proc)
		}
	}
	c.ll.Init()
	c.items = make(map[uuid.UUID]*list.Element)
	c.mu.Unlock()
	c.closeAll(procs)
	return nil
}

// Len reports how many instances are resident (built or building).
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

// closeAll closes any processors that hold resources, logging failures — a
// close error must not fail the turn that triggered the eviction.
func (c *Cache) closeAll(procs []chats.AgentProcessor) {
	for _, p := range procs {
		closer, ok := p.(io.Closer)
		if !ok {
			continue
		}
		if err := closer.Close(); err != nil {
			c.logger.Error("close evicted agent instance", "error", err)
		}
	}
}

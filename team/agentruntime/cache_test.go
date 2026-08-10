package agentruntime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/team/chats"
)

// fakeProc is a test processor that records whether it was closed.
type fakeProc struct {
	id     uuid.UUID
	closed atomic.Bool
}

func (p *fakeProc) Process(context.Context, string, string) (string, error) { return "ok", nil }
func (p *fakeProc) Close() error                                            { p.closed.Store(true); return nil }

// fakeLoader serves configs from a map and counts reads.
type fakeLoader struct {
	mu        sync.Mutex
	configs   map[uuid.UUID]AgentConfig
	slugErr   error
	loadErr   error
	loadCalls int
	slugCalls int
}

func newFakeLoader() *fakeLoader {
	return &fakeLoader{configs: make(map[uuid.UUID]AgentConfig)}
}

func (l *fakeLoader) add(cfg AgentConfig) { l.configs[cfg.ID] = cfg }

func (l *fakeLoader) AgentSlug(_ context.Context, id uuid.UUID) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.slugCalls++
	if l.slugErr != nil {
		return "", l.slugErr
	}
	cfg, ok := l.configs[id]
	if !ok {
		return "", fmt.Errorf("not found")
	}
	return cfg.Slug, nil
}

func (l *fakeLoader) LoadConfig(_ context.Context, id uuid.UUID) (AgentConfig, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.loadCalls++
	if l.loadErr != nil {
		return AgentConfig{}, l.loadErr
	}
	cfg, ok := l.configs[id]
	if !ok {
		return AgentConfig{}, fmt.Errorf("not found")
	}
	return cfg, nil
}

// fakeBuilder returns a fresh fakeProc per call and counts builds. failFirst,
// when >0, makes the first N builds fail.
type fakeBuilder struct {
	mu        sync.Mutex
	builds    int
	failFirst int
	procs     []*fakeProc
}

func (b *fakeBuilder) Build(_ context.Context, cfg AgentConfig) (chats.AgentProcessor, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.builds++
	if b.failFirst > 0 {
		b.failFirst--
		return nil, fmt.Errorf("build failed")
	}
	p := &fakeProc{id: cfg.ID}
	b.procs = append(b.procs, p)
	return p, nil
}

func (b *fakeBuilder) buildCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.builds
}

func newCache(t *testing.T, l ConfigLoader, b Builder, max int) *Cache {
	t.Helper()
	c, err := New(Config{Loader: l, Builder: b, MaxInstances: max})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestNew_Validation(t *testing.T) {
	if _, err := New(Config{Builder: &fakeBuilder{}}); err == nil {
		t.Error("New with nil loader should error")
	}
	if _, err := New(Config{Loader: newFakeLoader()}); err == nil {
		t.Error("New with nil builder should error")
	}
}

func TestProcessor_LazyBuildAndCache(t *testing.T) {
	l := newFakeLoader()
	id := uuid.New()
	l.add(AgentConfig{ID: id, Slug: "a"})
	b := &fakeBuilder{}
	c := newCache(t, l, b, 8)
	ctx := context.Background()

	p1, err := c.Processor(ctx, id)
	if err != nil {
		t.Fatalf("Processor: %v", err)
	}
	p2, err := c.Processor(ctx, id)
	if err != nil {
		t.Fatalf("Processor (cached): %v", err)
	}
	if p1 != p2 {
		t.Error("second Processor returned a different instance")
	}
	if b.buildCount() != 1 {
		t.Errorf("builds = %d, want 1 (lazy, cached)", b.buildCount())
	}
	if c.Len() != 1 {
		t.Errorf("Len = %d, want 1", c.Len())
	}
}

func TestProcessor_Concurrent_SingleBuild(t *testing.T) {
	l := newFakeLoader()
	id := uuid.New()
	l.add(AgentConfig{ID: id, Slug: "a"})
	b := &fakeBuilder{}
	c := newCache(t, l, b, 8)
	ctx := context.Background()

	const n = 32
	var wg sync.WaitGroup
	procs := make([]chats.AgentProcessor, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p, err := c.Processor(ctx, id)
			if err != nil {
				t.Errorf("Processor: %v", err)
				return
			}
			procs[i] = p
		}(i)
	}
	wg.Wait()

	if b.buildCount() != 1 {
		t.Errorf("builds = %d, want 1 (concurrent callers share one build)", b.buildCount())
	}
	for i := 1; i < n; i++ {
		if procs[i] != procs[0] {
			t.Fatalf("goroutine %d got a different instance", i)
		}
	}
}

func TestProcessor_LRUEviction_ClosesEvicted(t *testing.T) {
	l := newFakeLoader()
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	for i, id := range ids {
		l.add(AgentConfig{ID: id, Slug: fmt.Sprintf("a%d", i)})
	}
	b := &fakeBuilder{}
	c := newCache(t, l, b, 2)
	ctx := context.Background()

	pA, _ := c.Processor(ctx, ids[0])
	_, _ = c.Processor(ctx, ids[1])
	_, _ = c.Processor(ctx, ids[2]) // evicts the LRU (ids[0])

	if c.Len() != 2 {
		t.Errorf("Len = %d, want 2 (bounded)", c.Len())
	}
	if !pA.(*fakeProc).closed.Load() {
		t.Error("evicted instance was not closed")
	}
	// Re-accessing the evicted agent rebuilds it.
	if _, err := c.Processor(ctx, ids[0]); err != nil {
		t.Fatalf("Processor after eviction: %v", err)
	}
	if b.buildCount() != 4 {
		t.Errorf("builds = %d, want 4 (3 + 1 rebuild)", b.buildCount())
	}
}

func TestProcessor_LRUOrder_MostRecentSurvives(t *testing.T) {
	l := newFakeLoader()
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	for i, id := range ids {
		l.add(AgentConfig{ID: id, Slug: fmt.Sprintf("a%d", i)})
	}
	b := &fakeBuilder{}
	c := newCache(t, l, b, 2)
	ctx := context.Background()

	pA, _ := c.Processor(ctx, ids[0])
	pB, _ := c.Processor(ctx, ids[1])
	// Touch A so it becomes most-recently-used; adding C should now evict B.
	if _, err := c.Processor(ctx, ids[0]); err != nil {
		t.Fatalf("touch A: %v", err)
	}
	_, _ = c.Processor(ctx, ids[2])

	if pB.(*fakeProc).closed.Load() != true {
		t.Error("B should have been evicted (LRU) and closed")
	}
	if pA.(*fakeProc).closed.Load() {
		t.Error("A was touched and must survive")
	}
}

func TestProcessor_BuildError_NotCached(t *testing.T) {
	l := newFakeLoader()
	id := uuid.New()
	l.add(AgentConfig{ID: id, Slug: "a"})
	b := &fakeBuilder{failFirst: 1}
	c := newCache(t, l, b, 8)
	ctx := context.Background()

	if _, err := c.Processor(ctx, id); err == nil {
		t.Fatal("expected build error on first call")
	}
	if c.Len() != 0 {
		t.Errorf("Len = %d after failed build, want 0 (poisoned entry dropped)", c.Len())
	}
	// Retry succeeds — the failure was not cached.
	if _, err := c.Processor(ctx, id); err != nil {
		t.Fatalf("retry after build error: %v", err)
	}
	if c.Len() != 1 {
		t.Errorf("Len = %d, want 1 after successful retry", c.Len())
	}
}

func TestProcessor_LoadError_NotCached(t *testing.T) {
	l := newFakeLoader()
	l.loadErr = errors.New("db down")
	b := &fakeBuilder{}
	c := newCache(t, l, b, 8)

	if _, err := c.Processor(context.Background(), uuid.New()); err == nil {
		t.Fatal("expected load error")
	}
	if c.Len() != 0 {
		t.Errorf("Len = %d after load error, want 0", c.Len())
	}
	if b.buildCount() != 0 {
		t.Errorf("builds = %d, want 0 (build not reached on load error)", b.buildCount())
	}
}

func TestSlug_CheapNoBuild(t *testing.T) {
	l := newFakeLoader()
	id := uuid.New()
	l.add(AgentConfig{ID: id, Slug: "mentionme"})
	b := &fakeBuilder{}
	c := newCache(t, l, b, 8)

	slug, err := c.Slug(context.Background(), id)
	if err != nil {
		t.Fatalf("Slug: %v", err)
	}
	if slug != "mentionme" {
		t.Errorf("slug = %q, want mentionme", slug)
	}
	if b.buildCount() != 0 {
		t.Errorf("Slug built an instance (builds=%d), want 0", b.buildCount())
	}
	if c.Len() != 0 {
		t.Errorf("Slug cached an instance (Len=%d), want 0", c.Len())
	}
}

func TestInvalidate(t *testing.T) {
	l := newFakeLoader()
	id := uuid.New()
	l.add(AgentConfig{ID: id, Slug: "a"})
	b := &fakeBuilder{}
	c := newCache(t, l, b, 8)
	ctx := context.Background()

	p, _ := c.Processor(ctx, id)
	c.Invalidate(id)
	if !p.(*fakeProc).closed.Load() {
		t.Error("Invalidate did not close the instance")
	}
	if c.Len() != 0 {
		t.Errorf("Len = %d after Invalidate, want 0", c.Len())
	}
	// Next turn rebuilds from current config.
	if _, err := c.Processor(ctx, id); err != nil {
		t.Fatalf("Processor after Invalidate: %v", err)
	}
	if b.buildCount() != 2 {
		t.Errorf("builds = %d, want 2 (rebuild after invalidate)", b.buildCount())
	}

	// Invalidating an absent agent is a no-op.
	c.Invalidate(uuid.New())
}

func TestClose(t *testing.T) {
	l := newFakeLoader()
	ids := []uuid.UUID{uuid.New(), uuid.New()}
	for i, id := range ids {
		l.add(AgentConfig{ID: id, Slug: fmt.Sprintf("a%d", i)})
	}
	b := &fakeBuilder{}
	c := newCache(t, l, b, 8)
	ctx := context.Background()

	pA, _ := c.Processor(ctx, ids[0])
	pB, _ := c.Processor(ctx, ids[1])
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !pA.(*fakeProc).closed.Load() || !pB.(*fakeProc).closed.Load() {
		t.Error("Close did not close all instances")
	}
	if c.Len() != 0 {
		t.Errorf("Len = %d after Close, want 0", c.Len())
	}
}

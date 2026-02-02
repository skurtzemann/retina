# Cache Unification Plan

## Goal

Replace two separate cache instances with a single shared cache instance used by:
- Controllers (Pod, Node, Service, RetinaEndpoint)
- Plugin manager (packetparser, DNS, etc.)
- Metrics module

## Problem Statement

Currently, Retina creates **two separate cache instances**:

| Cache Instance | Location | Config Set | Used By | Global Access |
|---------------|----------|------------|---------|---------------|
| **Local Controller Cache** | `cmd/standard/daemon.go:242` | No (cfg=nil) | Pod, Node, Service, RetinaEndpoint controllers | No |
| **Global Cache** | `pkg/managers/controllermanager/controllermanager.go:83` | Yes | Plugin manager, Metrics module | Yes (`cache.GlobalCache`) |

### Implications of Two Caches

- **Duplicate data**: Same pod/node entries stored twice
- **No synchronization**: Changes in one cache don't reflect in the other
- **Debug logs missing**: Local cache never logs even when `enableCacheDebugLog: true`
- **Memory overhead**: Duplicate entries consume extra memory

## Current Architecture

```
cmd/standard/daemon.go          pkg/managers/controllermanager/
        │                               │
   controllerCache              m.cache → GlobalCache
        │                               │
        ▼                               ▼
   Controllers             Plugins/Metrics (use GlobalCache)
```

## Proposed Architecture

```
cmd/standard/daemon.go
        │
        ▼
   controllerCache ──► GlobalCache (single source of truth)
        │
        ▼
   All components (controllers, plugins, metrics)
```

## Implementation Steps

### Phase 1: Update ControllerManager.Init() Signature

**File:** `pkg/managers/controllermanager/controllermanager.go`

**Change:** Accept `*cache.Cache` as parameter instead of creating new cache

```go
// Before
func (m *Controller) Init(ctx context.Context) error {
    m.l.Info("Initializing controller manager ...")

    if err := m.httpServer.Init(); err != nil {
        return err
    }

    if m.conf.EnablePodLevel {
        m.pubsub = pubsub.New()
        m.cache = cache.New(m.pubsub)      // Creates new cache
        m.cache.SetConfig(m.conf)
        cache.GlobalCache = m.cache
        m.enricher = enricher.New(ctx, m.cache)
    }

    return nil
}

// After
func (m *Controller) Init(ctx context.Context, controllerCache *cache.Cache) error {
    m.l.Info("Initializing controller manager ...")

    if err := m.httpServer.Init(); err != nil {
        return err
    }

    if m.conf.EnablePodLevel {
        m.cache = controllerCache  // Use passed cache
        cache.GlobalCache = m.cache
        m.enricher = enricher.New(ctx, m.cache)
    }

    return nil
}
```

### Phase 2: Create Cache Once in Daemon

**File:** `cmd/standard/daemon.go`

**Change:** Create cache once, pass to ControllerManager

```go
// Before
if daemonConfig.EnablePodLevel {
    pubSub := pubsub.New()
    controllerCache := controllercache.New(pubSub)
    controllerCache.SetConfig(daemonConfig)
    enrich := enricher.New(ctx, controllerCache)
    // ... rest of setup
}

// After
if daemonConfig.EnablePodLevel {
    pubSub := pubsub.New()
    controllerCache := controllercache.New(pubSub)
    controllerCache.SetConfig(daemonConfig)
    cache.GlobalCache = controllerCache  // Set global cache
    enrich := enricher.New(ctx, controllerCache)
    // ... rest of setup
}

// Update ControllerManager.Init call
if err := controllerMgr.Init(ctx, controllerCache); err != nil {
    mainLogger.Fatal("Failed to initialize controller manager", zap.Error(err))
}
```

### Phase 3: Update All Callers

Find and update all callers of `NewControllerManager()`:

```go
// Find callers
grep -r "NewControllerManager" --include="*.go"
```

Update function signatures and calls to pass the cache instance.

### Phase 4: Remove Redundant Code

Remove redundant cache creation in `ControllerManager.Init()` (already done in Phase 1).

## Files to Modify

| File | Change |
|------|--------|
| `pkg/managers/controllermanager/controllermanager.go` | Change `Init()` signature to accept cache |
| `pkg/managers/controllermanager/controllermanager.go` | Remove redundant cache creation |
| `cmd/standard/daemon.go` | Create cache once, pass to ControllerManager |
| Any other files calling `NewControllerManager()` | Update function calls |

## Benefits

| Benefit | Description |
|---------|-------------|
| **Single source of truth** | No duplicate entries |
| **Consistent state** | Controllers and plugins see same data |
| **Memory savings** | No duplicate cache structures |
| **Simpler debugging** | One cache to inspect |
| **Consistent logging** | All operations log the same way |

## Rollback Plan

1. Keep the original `Init()` signature as deprecated first
2. Test with the new signature
3. If issues arise, revert to old signature (no functional change, just back to two caches)

## Testing Plan

1. Verify cache statistics show consistent counts
2. Verify pod/node logs appear in both controllers and plugins
3. Verify metrics are calculated correctly (using shared cache)
4. Test pod/node CRUD operations and verify cache updates reflect everywhere

## Timeline

- **Phase 1-2:** Core implementation (1-2 hours)
- **Phase 3:** Find and update callers (varies)
- **Phase 4:** Cleanup (30 minutes)
- **Testing:** 1-2 hours

Total estimated time: 4-6 hours

## References

- TODO comment acknowledging temporary solution: `cmd/standard/daemon.go:236`
- Existing cache debug logging: `pkg/controllers/cache/cache.go`
- GlobalCache singleton: `pkg/controllers/cache/cache.go:17`


```

Before (Two Separate Caches)
cmd/standard/daemon.go:242          pkg/managers/controllermanager/
│                                │
controllerCache ──────────────►  m.cache → GlobalCache
│                                │
▼                                ▼
Controllers                    Metrics/Plugins

After (Single Shared Cache)
cmd/standard/daemon.go:242
│
▼
controllerCache ──────────────────────────────► GlobalCache (same instance)
│                                              │
└── Controllers, Plugins, Metrics all share ───┘
```
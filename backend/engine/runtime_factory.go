package engine

import (
	"fmt"
	"strings"
)

// DefaultAdapterID is the runtime used when no explicit runtime kind is
// configured. I008 ships only the ADK adapter; later iterations register more.
const DefaultAdapterID = ADKAdapterID

// DefaultAdapterRegistry returns the registry holding every runtime adapter
// compiled into this binary. The Pi subprocess adapter is registered only when
// its feature flag is enabled (OPSK_ENABLE_PI_RUNTIME=1, I008 C1-4); with the
// flag off the Pi layer is entirely disabled and the registry is identical to
// the pre-I008-M3 rollout (NFR-004).
func DefaultAdapterRegistry(buildModel ModelBuilder) (*AdapterRegistry, error) {
	registry := NewAdapterRegistry()
	if err := registry.Register(NewADKAdapter(buildModel)); err != nil {
		return nil, fmt.Errorf("register adk engine adapter: %w", err)
	}
	if PiRuntimeEnabled() {
		// Startup orphan sweep (FR-013): any process group left registered by a
		// previous crashed process is force-killed before new work starts.
		ReapPiOrphans()
		if err := registry.Register(NewPiAdapter(PiProcessDefaults())); err != nil {
			return nil, fmt.Errorf("register pi engine adapter: %w", err)
		}
	}
	return registry, nil
}

// ExperimentalAdapter reports whether the adapter is registered behind an
// explicit feature flag and must never become a scenario default runtime.
type ExperimentalAdapter interface {
	Experimental() bool
}

// AdapterIsExperimental reports whether the given adapter declared itself
// experimental.
func AdapterIsExperimental(adapter EngineAdapter) bool {
	if experimental, ok := adapter.(ExperimentalAdapter); ok {
		return experimental.Experimental()
	}
	return false
}

// RunnerProvider lets an adapter expose the platform Runner implementation the
// Runtime drives today. Later iterations move the Runtime fully onto the
// adapter contract; until then this is the bridge that keeps the platform
// execution path identical for every wiring site.
type RunnerProvider interface {
	Runner() Runner
}

// ResolveEngineRunner resolves the configured runtime kind through the adapter
// registry and returns the platform Runner for the resolved adapter. Unknown,
// empty or not-yet-executable runtime kinds fall back to the default adapter;
// the returned note explains every fallback so callers can leave an audit
// trace (B3-3: 配置错误回退默认运行时并留痕).
func ResolveEngineRunner(registry *AdapterRegistry, kind string, buildModel ModelBuilder) (Runner, EngineAdapter, string) {
	id := strings.TrimSpace(kind)
	if id == "" {
		id = DefaultAdapterID
	}
	if registry != nil {
		if adapter, ok := registry.Get(id); ok {
			if provider, isProvider := adapter.(RunnerProvider); isProvider {
				if runner := provider.Runner(); runner != nil {
					return runner, adapter, ""
				}
			}
			if id != DefaultAdapterID {
				runner, adapter, note := defaultEngineRunner(registry, buildModel)
				return runner, adapter, fmt.Sprintf("runtime %q is registered but provides no runner yet; fell back to %q", id, DefaultAdapterID) + note
			}
		} else if id != DefaultAdapterID {
			runner, adapter, note := defaultEngineRunner(registry, buildModel)
			return runner, adapter, fmt.Sprintf("runtime kind %q is not registered; fell back to %q", id, DefaultAdapterID) + note
		}
	}
	return defaultEngineRunner(registry, buildModel)
}

func defaultEngineRunner(registry *AdapterRegistry, buildModel ModelBuilder) (Runner, EngineAdapter, string) {
	if registry != nil {
		if adapter, ok := registry.Get(DefaultAdapterID); ok {
			if provider, isProvider := adapter.(RunnerProvider); isProvider {
				if runner := provider.Runner(); runner != nil {
					return runner, adapter, ""
				}
			}
		}
	}
	adapter := NewADKAdapter(buildModel)
	return adapter.Runner(), adapter, " (constructed without registry)"
}

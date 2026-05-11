package scenarios

import (
	"fmt"
	"sort"
)

// Registry resolves a scenario name (as carried on the HTTP request) to
// the implementation. Built once at startup from the enabled-scenarios
// config; subsequent lookups are O(1).
type Registry struct {
	byName map[string]Scenario
}

// NewRegistry constructs a registry from the supplied scenarios, filtered
// by the enabled list. An enabled name with no implementation is a config
// error and fails startup.
func NewRegistry(impls []Scenario, enabled []string) (*Registry, error) {
	byName := map[string]Scenario{}
	for _, s := range impls {
		byName[s.Name()] = s
	}
	out := &Registry{byName: map[string]Scenario{}}
	for _, name := range enabled {
		s, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("scenario %q enabled in config but no implementation registered", name)
		}
		out.byName[name] = s
	}
	return out, nil
}

// Lookup returns the scenario for the given name, or an error if it is
// not registered (either not implemented or disabled by config).
func (r *Registry) Lookup(name string) (Scenario, error) {
	s, ok := r.byName[name]
	if !ok {
		return nil, fmt.Errorf("scenario %q is not enabled or not implemented", name)
	}
	return s, nil
}

// Names returns the registered scenario names sorted for stable logging.
func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.byName))
	for k := range r.byName {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

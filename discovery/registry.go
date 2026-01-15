// Package discovery is the registry for discovery.
package discovery

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/registry"

	discoveryv1 "github.com/aide-family/goddess/pkg/discovery/v1"
)

var globalRegistry = NewRegistry()

type Factory func(discoveryConfig *discoveryv1.Discovery) (registry.Discovery, error)

// Registry is the interface for callers to get registered discovery.
type Registry interface {
	Register(name string, factory Factory)
	Create(discoveryConfig *discoveryv1.Discovery) (registry.Discovery, error)
}

type discoveryRegistry struct {
	discovery map[string]Factory
}

// NewRegistry returns a new discovery registry.
func NewRegistry() Registry {
	return &discoveryRegistry{
		discovery: map[string]Factory{},
	}
}

func (d *discoveryRegistry) Register(name string, factory Factory) {
	d.discovery[name] = factory
}

func (d *discoveryRegistry) Create(discoveryConfig *discoveryv1.Discovery) (registry.Discovery, error) {
	if discoveryConfig == nil {
		return nil, nil
	}
	if discoveryConfig.Required && discoveryConfig.Name == "" {
		return nil, fmt.Errorf("discovery is required")
	}
	factory, ok := d.discovery[discoveryConfig.Name]
	if !ok {
		return nil, fmt.Errorf("discovery %s has not been registered", discoveryConfig.Name)
	}

	impl, err := factory(discoveryConfig)
	if err != nil {
		return nil, fmt.Errorf("create discovery error: %s", err)
	}
	return impl, nil
}

// Register registers one discovery.
func Register(name string, factory Factory) {
	globalRegistry.Register(name, factory)
}

// Create instantiates a discovery based on `discoveryDSN`.
func Create(discoveryConfig *discoveryv1.Discovery) (registry.Discovery, error) {
	return globalRegistry.Create(discoveryConfig)
}

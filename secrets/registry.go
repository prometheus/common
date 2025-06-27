package secrets

import "fmt"

type providerFactory = func() Provider

type ProviderRegistry struct {
	map_ map[string]providerFactory
}

func (r *ProviderRegistry) Get(name string) (Provider, error) {
	if constructor, ok := r.map_[name]; ok {
		return constructor(), nil
	}
	return nil, fmt.Errorf("unknown provider type: %q", name)
}

func (r *ProviderRegistry) Register(constructor func() Provider) {
	name := constructor().Name()
	if _, ok := r.map_[name]; ok {
		panic(fmt.Sprintf("attempt to register duplicate type: %q", name))
	}
	if r.map_ == nil {
		r.map_ = make(map[string]providerFactory)
	}
	r.map_[name] = constructor
}

var Providers *ProviderRegistry = &ProviderRegistry{}

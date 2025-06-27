// Copyright 2025 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package secrets

import "fmt"

type providerFactory = func() Provider

type ProviderRegistry struct {
	factoryMap map[string]providerFactory
}

func (r *ProviderRegistry) Get(name string) (Provider, error) {
	if constructor, ok := r.factoryMap[name]; ok {
		return constructor(), nil
	}
	return nil, fmt.Errorf("unknown provider type: %q", name)
}

func (r *ProviderRegistry) Register(constructor func() Provider) {
	name := constructor().Name()
	if _, ok := r.factoryMap[name]; ok {
		panic(fmt.Sprintf("attempt to register duplicate type: %q", name))
	}
	if r.factoryMap == nil {
		r.factoryMap = make(map[string]providerFactory)
	}
	r.factoryMap[name] = constructor
}

var Providers = &ProviderRegistry{}

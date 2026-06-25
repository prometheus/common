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

import (
	"context"
	"sync"
)

type MockProvider struct {
	Secret        string `yaml:"secret"`
	ProviderID    string `yaml:"provider_id"`
	mtx           *sync.Mutex
	fetchErr      error
	fetchedLatest bool
	blockChan     chan struct{}
	releaseChan   chan struct{}
}

func (mp *MockProvider) FetchSecret(ctx context.Context) (string, error) {
	// Block if the test requires it, to simulate fetch latency.
	if mp.blockChan != nil {
		select {
		case <-mp.blockChan:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	// Release if the test requires it, to signal fetch has started.
	if mp.releaseChan != nil {
		close(mp.releaseChan)
	}

	mp.mtx.Lock()
	defer mp.mtx.Unlock()
	mp.fetchedLatest = true
	return mp.Secret, mp.fetchErr
}

func (mp *MockProvider) SetSecret(s string) {
	mp.mtx.Lock()
	defer mp.mtx.Unlock()
	mp.fetchedLatest = false
	mp.Secret = s
}

func (mp *MockProvider) SetFetchError(err error) {
	mp.mtx.Lock()
	defer mp.mtx.Unlock()
	mp.fetchedLatest = false
	mp.fetchErr = err
}

func (mp *MockProvider) HasFetchedLatest() bool {
	mp.mtx.Lock()
	defer mp.mtx.Unlock()
	return mp.fetchedLatest
}

func (mp *MockProvider) NewProvider() (Provider, error) {
	return mp, nil
}

func (mp *MockProvider) FromString(s string) {
	mp.Secret = s
}

func (mp *MockProvider) ID() string {
	if len(mp.ProviderID) > 0 {
		return mp.ProviderID
	}
	return mp.Secret
}

func (mp *MockProvider) Clone() ProviderConfig {
	mtx := mp.mtx
	if mtx == nil {
		mtx = &sync.Mutex{}
	}
	return &MockProvider{
		Secret:        mp.Secret,
		fetchErr:      mp.fetchErr,
		mtx:           mtx,
		fetchedLatest: mp.fetchedLatest,
		blockChan:     mp.blockChan,
		releaseChan:   mp.releaseChan,
	}
}

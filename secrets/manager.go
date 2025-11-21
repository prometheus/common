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
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	// fetchTimeout governs the maximum time a single fetch attempt can take.
	fetchTimeout = 5 * time.Minute
	// fetchInitialBackoff is the initial backoff duration for refetching a secret after a failure.
	fetchInitialBackoff = 1 * time.Second
	// fetchMaxBackoff is the maximum backoff duration for retrying a failed fetch.
	fetchMaxBackoff = 2 * time.Minute

	// the default refresh interval for secrets.
	defaultRefreshInterval = time.Hour

	// Prometheus secret states.
	stateSuccess      float64 = 0
	stateStale        float64 = 1
	stateError        float64 = 2
	stateInitializing float64 = 3
)

// Manager discovers, manages, and refreshes all Field instances within a
// given configuration.
//
// After unmarshaling a configuration file into a struct, a new Manager should
// be created by passing a pointer to the configuration struct to NewManager.
// The Manager will discover all fields of type Field, parse their
// configuration, and prepare them for fetching.
//
// The Manager should be started by calling the Start method, which launches a
// background goroutine to periodically refresh the secrets. The Stop method
// should be called to stop the background refresh.
//
// Before accessing any secret values, the SecretsReady method should be
// called to ensure that all secrets have been successfully fetched at least
// once.
type Manager struct {
	mtx       sync.RWMutex
	secrets   map[string]*managedSecret
	providers *ProviderRegistry
	refreshC  chan struct{}
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	// Prometheus metrics
	lastSuccessfulFetch *prometheus.GaugeVec
	secretState         *prometheus.GaugeVec
	fetchSuccessTotal   *prometheus.CounterVec
	fetchFailuresTotal  *prometheus.CounterVec
	fetchDuration       *prometheus.HistogramVec
}

type managedSecret struct {
	mtx              sync.RWMutex
	secret           string
	fetched          time.Time
	fetchInProgress  bool
	refreshInterval  time.Duration
	refreshRequested bool
	metricLabels     prometheus.Labels
	provider         Provider
}

// NewManager discovers all Field instances within the provided config
// structure using reflection and registers them with this manager.
//
// It also registers Prometheus metrics to monitor the state of the secrets.
// The following metrics are available, all labeled with `provider` and `secret_id`:
//
//   - `prometheus_remote_secret_last_successful_fetch_seconds`: (Gauge) The Unix
//     timestamp of the last successful secret fetch.
//   - `prometheus_remote_secret_state`: (Gauge) Describes the current state of a
//     remotely fetched secret (0=success, 1=stale, 2=error, 3=initializing).
//   - `prometheus_remote_secret_fetch_success_total`: (Counter) Total number of
//     successful secret fetches.
//   - `prometheus_remote_secret_fetch_failures_total`: (Counter) Total number
//     of failed secret fetches.
//   - `prometheus_remote_secret_fetch_duration_seconds`: (Histogram) Duration of
//     secret fetch attempts.
func NewManager(r prometheus.Registerer, providers *ProviderRegistry, config interface{}) (*Manager, error) {
	paths, err := getSecretFields(config)
	if err != nil {
		return nil, err
	}
	manager := &Manager{
		secrets:   make(map[string]*managedSecret),
		providers: providers,
	}
	manager.registerMetrics(r)
	for path, field := range paths {
		if err := manager.registerSecret(path, field); err != nil {
			return nil, err
		}
	}
	return manager, nil
}

func (m *Manager) registerMetrics(r prometheus.Registerer) {
	labels := []string{"provider", "secret_id"}

	m.lastSuccessfulFetch = promauto.With(r).NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "prometheus_remote_secret_last_successful_fetch_seconds",
			Help: "The unix timestamp of the last successful secret fetch.",
		},
		labels,
	)
	m.secretState = promauto.With(r).NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "prometheus_remote_secret_state",
			Help: "Describes the current state of a remotely fetched secret (0=success, 1=stale, 2=error, 3=initializing).",
		},
		labels,
	)
	m.fetchSuccessTotal = promauto.With(r).NewCounterVec(
		prometheus.CounterOpts{
			Name: "prometheus_remote_secret_fetch_success_total",
			Help: "Total number of successful secret fetches.",
		},
		labels,
	)
	m.fetchFailuresTotal = promauto.With(r).NewCounterVec(
		prometheus.CounterOpts{
			Name: "prometheus_remote_secret_fetch_failures_total",
			Help: "Total number of failed secret fetches.",
		},
		labels,
	)

	m.fetchDuration = promauto.With(r).NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "prometheus_remote_secret_fetch_duration_seconds",
			Help:    "Duration of secret fetch attempts.",
			Buckets: prometheus.DefBuckets,
		},
		labels,
	)
}

func (m *Manager) registerSecret(path string, s *Field) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	state, err := s.parseRawConfig(m.providers, path)
	if err != nil {
		return err
	}

	labels := prometheus.Labels{
		"provider":  state.providerName,
		"secret_id": state.id(),
	}

	if state.settings.RefreshInterval == 0 {
		state.settings.RefreshInterval = defaultRefreshInterval
	}

	if _, exists := m.secrets[state.id()]; !exists {
		provider, err := state.config.NewProvider()
		if err != nil {
			return err
		}
		ms := &managedSecret{
			refreshInterval: state.settings.RefreshInterval,
			metricLabels:    labels,
			provider:        provider,
		}
		m.secrets[state.id()] = ms
		m.secretState.With(labels).Set(stateInitializing)
		m.fetchSuccessTotal.With(labels).Add(0)
		m.fetchFailuresTotal.With(labels).Add(0)
	}
	m.secrets[state.id()].mtx.Lock()
	defer m.secrets[state.id()].mtx.Unlock()

	m.secrets[state.id()].refreshInterval = min(
		m.secrets[state.id()].refreshInterval,
		state.settings.RefreshInterval)
	s.state = state
	return nil
}

func (m *Manager) secretReady(s *Field) bool {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	ms := m.secrets[s.state.id()]
	ms.mtx.Lock()
	defer ms.mtx.Unlock()

	s.state.mutex.Lock()
	defer s.state.mutex.Unlock()

	ready := !ms.fetched.IsZero()
	if ready {
		s.state.value = ms.secret
	}
	if s.state.requestRefresh {
		s.state.requestRefresh = false
		ms.refreshRequested = true
		select {
		case m.refreshC <- struct{}{}:
		default:
			// a refresh is already pending, do nothing
		}
	}
	return ready
}

// SecretsReady checks if all secrets in the provided config have been
// successfully fetched at least once.
//
// This method should be called before accessing any secret values to ensure
// that they are available.
func (m *Manager) SecretsReady(config interface{}) (bool, error) {
	paths, err := getSecretFields(config)
	if err != nil {
		return false, err
	}
	allReady := true
	for _, field := range paths {
		if !m.secretReady(field) {
			allReady = false
		}
	}
	return allReady, nil
}

// Start launches the background goroutine that periodically fetches secrets.
func (m *Manager) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.fetchSecretsLoop(ctx)
	}()

	m.cancel = cancel
}

// Stop terminates the background secret fetching goroutine.
func (m *Manager) Stop() {
	m.cancel()
	m.wg.Wait()
}

// fetchSecretsLoop is a long-running goroutine that periodically fetches secrets.
func (m *Manager) fetchSecretsLoop(ctx context.Context) {
	timer := time.NewTimer(time.Duration(0))
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		case <-m.refreshC:
			if !timer.Stop() {
				<-timer.C
			}
		}
		m.mtx.RLock()
		// Create a list of secrets to check to avoid holding the lock during fetch operations.
		secretsToCheck := make([]*managedSecret, 0, len(m.secrets))
		for _, secret := range m.secrets {
			secretsToCheck = append(secretsToCheck, secret)
		}
		m.mtx.RUnlock()

		waitTime := 5 * time.Minute

		for _, ms := range secretsToCheck {
			ms.mtx.Lock()

			timeToRefresh := time.Until(ms.fetched.Add(ms.refreshInterval))
			refreshNeeded := ms.refreshRequested || timeToRefresh < 0
			waitTime = min(waitTime, ms.refreshInterval)

			if ms.fetchInProgress {
				ms.mtx.Unlock()
				continue
			}

			if !refreshNeeded {
				ms.mtx.Unlock()
				if timeToRefresh > 0 {
					waitTime = min(waitTime, timeToRefresh)
				}
				continue
			}
			ms.fetchInProgress = true
			ms.mtx.Unlock()

			go m.fetchAndStoreSecret(ctx, ms)
		}
		timer.Reset(waitTime)
	}
}

// fetchAndStoreSecret performs a single secret fetch, including retry logic with exponential backoff.
// It is robust against hangs in the underlying provider's FetchSecret method.
func (m *Manager) fetchAndStoreSecret(ctx context.Context, ms *managedSecret) {
	var newSecret string
	var err error
	ms.mtx.RLock()
	provider := ms.provider
	labels := ms.metricLabels
	hasBeenFetchedBefore := !ms.fetched.IsZero()
	ms.mtx.RUnlock()

	backoff := fetchInitialBackoff
	for {
		fetchCtx, cancel := context.WithTimeout(ctx, fetchTimeout)

		newSecret, err = provider.FetchSecret(fetchCtx)
		cancel()

		if err == nil {
			break // Success
		}

		m.fetchFailuresTotal.With(labels).Inc()
		if hasBeenFetchedBefore {
			m.secretState.With(labels).Set(stateStale)
		} else {
			m.secretState.With(labels).Set(stateError)
		}

		select {
		case <-time.After(backoff):
			backoff = min(fetchMaxBackoff, backoff*2)
		case <-ctx.Done():
			return
		}
	}
	ms.mtx.Lock()

	m.fetchSuccessTotal.With(labels).Inc()
	m.lastSuccessfulFetch.With(labels).SetToCurrentTime()
	m.secretState.With(labels).Set(stateSuccess)

	ms.secret = newSecret
	ms.fetched = time.Now()
	ms.fetchInProgress = false
	ms.refreshRequested = false
	ms.mtx.Unlock()
}

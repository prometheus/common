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
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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

type Manager struct {
	MarshalInlineSecrets bool
	mtx                  sync.RWMutex
	secrets              map[*SecretField]*managedSecret
	refreshC             chan struct{}
	allReady             atomic.Bool
	cancel               context.CancelFunc
	wg                   sync.WaitGroup
	// Prometheus metrics
	lastSuccessfulFetch     *prometheus.GaugeVec
	secretState             *prometheus.GaugeVec
	fetchSuccessTotal       *prometheus.CounterVec
	fetchFailuresTotal      *prometheus.CounterVec
	fetchDuration           *prometheus.HistogramVec
	validationFailuresTotal *prometheus.CounterVec
}

type managedSecret struct {
	mtx              sync.RWMutex
	pendingSecret    string
	secret           string
	provider         Provider
	fetched          time.Time
	fetchInProgress  bool
	refreshInterval  time.Duration
	refreshRequested bool
	validator        SecretValidator
	verified         bool
	metricLabels     prometheus.Labels
}

// NewManager discovers all SecretField instances within the provided config
// structure using reflection and registers them with this manager.
func NewManager(r prometheus.Registerer, config interface{}) (*Manager, error) {
	paths, err := getSecretFields(config)
	if err != nil {
		return nil, err
	}
	manager := &Manager{
		secrets: make(map[*SecretField]*managedSecret),
	}
	manager.registerMetrics(r)
	for path, field := range paths {
		manager.registerSecret(path, field)
	}
	return manager, nil
}

func (m *Manager) registerMetrics(r prometheus.Registerer) {
	labels := []string{"provider", "secret_id"}

	m.lastSuccessfulFetch = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "prometheus_remote_secret_last_successful_fetch_seconds",
			Help: "The unix timestamp of the last successful secret fetch.",
		},
		labels,
	)
	m.secretState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "prometheus_remote_secret_state",
			Help: "Describes the current state of a remotely fetched secret (0=success, 1=stale, 2=error, 3=initializing).",
		},
		labels,
	)
	m.fetchSuccessTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "prometheus_remote_secret_fetch_success_total",
			Help: "Total number of successful secret fetches.",
		},
		labels,
	)
	m.fetchFailuresTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "prometheus_remote_secret_fetch_failures_total",
			Help: "Total number of failed secret fetches.",
		},
		labels,
	)

	m.fetchDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "prometheus_remote_secret_fetch_duration_seconds",
			Help:    "Duration of secret fetch attempts.",
			Buckets: prometheus.DefBuckets,
		},
		labels,
	)
	m.validationFailuresTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "prometheus_remote_secret_validation_failures_total",
			Help: "Total number of failed secret validations.",
		},
		labels,
	)

	// Register all metrics with the provided registry
	r.MustRegister(
		m.lastSuccessfulFetch,
		m.secretState,
		m.fetchSuccessTotal,
		m.fetchFailuresTotal,
		m.fetchDuration,
		m.validationFailuresTotal,
	)
}

func (m *Manager) registerSecret(path string, s *SecretField) {
	s.manager = m

	m.mtx.Lock()
	defer m.mtx.Unlock()

	labels := prometheus.Labels{
		"provider":  s.provider.Name(),
		"secret_id": path,
	}

	refreshInterval := s.settings.RefreshInterval
	if refreshInterval == 0 {
		refreshInterval = defaultRefreshInterval
	}

	ms := &managedSecret{
		provider:        s.provider,
		validator:       s.validator,
		refreshInterval: refreshInterval,
		metricLabels:    labels,
	}
	m.secrets[s] = ms
	m.secretState.With(labels).Set(stateInitializing)
	m.fetchSuccessTotal.With(labels).Add(0)
	m.fetchFailuresTotal.With(labels).Add(0)
	m.validationFailuresTotal.With(labels).Add(0)
}

func (m *Manager) secretReady(s *SecretField) bool {
	m.mtx.RLock()
	defer m.mtx.RUnlock()
	return !m.secrets[s].fetched.IsZero()
}

func (m *Manager) SecretsReady(config interface{}) (bool, error) {
	if m.allReady.Load() {
		return true, nil
	}
	paths, err := getSecretFields(config)
	if err != nil {
		return false, err
	}
	for _, field := range paths {
		if !m.secretReady(field) {
			return false, nil
		}
	}
	return true, nil
}

func (m *Manager) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.fetchSecretsLoop(ctx)
	}()

	m.cancel = cancel
}

func (m *Manager) Stop() {
	m.cancel()
	m.wg.Wait()
}

func (m *Manager) setSecretValidation(s *SecretField, validator SecretValidator) {
	m.mtx.RLock()
	secret := m.secrets[s]
	m.mtx.RUnlock()
	secret.mtx.Lock()
	secret.validator = validator
	secret.mtx.Unlock()
}

func (m *Manager) get(s *SecretField) string {
	if inline, ok := s.provider.(*InlineProvider); ok {
		return inline.secret
	}
	m.mtx.RLock()
	secret := m.secrets[s]
	m.mtx.RUnlock()
	secret.mtx.RLock()
	defer secret.mtx.RUnlock()
	return secret.secret
}

func (m *Manager) triggerRefresh(s *SecretField) {
	m.mtx.RLock()
	secret := m.secrets[s]
	m.mtx.RUnlock()
	secret.mtx.Lock()
	defer secret.mtx.Unlock()
	secret.refreshRequested = true
	secret.verified = false
	secret.secret = secret.pendingSecret
	select {
	case m.refreshC <- struct{}{}:
	default:
		// a refresh is already pending, do nothing
	}
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
		secretsReady := true

		for _, ms := range secretsToCheck {
			ms.mtx.Lock()

			timeToRefresh := time.Until(ms.fetched.Add(ms.refreshInterval))
			refreshNeeded := ms.refreshRequested || timeToRefresh < 0
			waitTime = min(waitTime, ms.refreshInterval)
			if ms.fetched.IsZero() {
				secretsReady = false
			}

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
		m.allReady.Store(secretsReady)
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

	ms.pendingSecret = newSecret
	ms.fetched = time.Now()
	ms.fetchInProgress = false
	ms.refreshRequested = false

	// If a was not verified before, we can swap it immediately
	if !ms.verified {
		ms.secret = newSecret
	}
	ms.mtx.Unlock()
	m.validateAndStoreField(ctx, ms, newSecret)
}

// validateAndStoreField performs validation for a single field, including retry logic.
func (m *Manager) validateAndStoreField(ctx context.Context, ms *managedSecret, pendingSecret string) {
	var isValid bool

	ms.mtx.RLock()
	var validator SecretValidator = DefaultValidator{}
	if ms.validator != nil {
		validator = ms.validator
	}
	labels := ms.metricLabels
	vs := validator.Settings()
	ms.mtx.RUnlock()

	backoff := vs.InitialBackoff
	for i := 0; i < vs.MaxRetries; i++ {
		ms.mtx.RLock()
		shouldRun := ms.pendingSecret == pendingSecret
		ms.mtx.RUnlock()
		if !shouldRun {
			return
		}
		validateCtx, cancel := context.WithTimeout(ctx, vs.Timeout)
		isValid = validator.Validate(validateCtx, pendingSecret)
		cancel()

		if isValid {
			break // Success
		}
		m.validationFailuresTotal.With(labels).Inc()
		if i < vs.MaxRetries-1 {
			select {
			case <-time.After(backoff):
				backoff = min(vs.MaxBackoff, backoff*2)
			case <-ctx.Done():
				return
			}
		}
	}

	ms.mtx.Lock()
	defer ms.mtx.Unlock()

	if ms.pendingSecret == pendingSecret {
		ms.secret = pendingSecret
		ms.verified = true
	}
}

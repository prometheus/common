package secrets

import (
	"context"
	"sync"
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

	// defaultValidateInterval is the default interval at which to check for pending secrets that need validation.
	defaultValidateInterval = 10 * time.Second
	// validationTimeout governs the maximum time a single validation attempt can take.
	validationTimeout = 30 * time.Second
	// validationInitialBackoff is the initial backoff duration for re-validating a secret after a failure.
	validationInitialBackoff = 1 * time.Second
	// validationMaxBackoff is the maximum backoff duration for retrying a failed validation.
	validationMaxBackoff = 30 * time.Second
	// validationMaxRetries is the maximum number of retries for a failed validation.
	validationMaxRetries = 10

	// Prometheus secret states
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
	allReady             bool
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
		"provider": s.provider.Name(),
		// Would this make external users dependant on
		// our config structure if they listen our metrics?
		// Basically a breaking change in config would also
		// become a breaking change for metrics?
		"secret_path": path,
	}

	ms := &managedSecret{
		provider: s.provider,
		// TODO: get refreshInterval from secretField
		refreshInterval: time.Hour,
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
	if m.allReady {
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

			refreshNeeded := ms.refreshRequested || time.Since(ms.fetched) > ms.refreshInterval
			timeToRefresh := time.Until(ms.fetched.Add(ms.refreshInterval))
			if ms.fetched.IsZero() {
				secretsReady = false
			}

			if ms.fetchInProgress {
				ms.mtx.Unlock()
				continue
			}

			if !refreshNeeded {
				waitTime = min(timeToRefresh, waitTime)
				ms.mtx.Unlock()
				continue
			}
			ms.fetchInProgress = true
			ms.mtx.Unlock()

			go m.fetchAndStoreSecret(ctx, ms)
		}
		m.allReady = secretsReady
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
	if ms.verified == false {
		ms.secret = newSecret
	}
	ms.verified = false
	ms.mtx.Unlock()
	m.validateAndStoreField(ctx, ms, newSecret)

}

// validateAndStoreField performs validation for a single field, including retry logic.
func (m *Manager) validateAndStoreField(ctx context.Context, ms *managedSecret, pendingSecret string) {
	var isValid bool

	ms.mtx.RLock()
	validator := ms.validator
	labels := ms.metricLabels
	ms.mtx.RUnlock()

	backoff := validationInitialBackoff
	for i := range validationMaxRetries {

		validateCtx, cancel := context.WithTimeout(ctx, validationTimeout)
		isValid = validator == nil || validator.Validate(validateCtx, pendingSecret)
		cancel()

		if isValid {
			break // Success
		}
		m.validationFailuresTotal.With(labels).Inc()
		if i < validationMaxRetries-1 {
			select {
			case <-time.After(backoff):
				backoff = min(validationMaxBackoff, backoff*2)
			case <-ctx.Done():
				return
			}
		}
	}

	ms.mtx.Lock()
	defer ms.mtx.Unlock()

	if ms.pendingSecret != pendingSecret {
		return
	}

	ms.secret = pendingSecret
	ms.verified = true
}

package secrets

import (
	"context"
	"sync"
	"time"
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
)

type Manager struct {
	MarshalInlineSecrets bool
	mtx                  sync.RWMutex
	secrets              map[string]*managedSecret
	refreshRequested     chan struct{}
	cancel               context.CancelFunc
}

type managedSecret struct {
	mtx              sync.RWMutex
	pendingSecret    string
	provider         Provider
	fetched          time.Time
	fetchInProgress  bool
	refreshInterval  time.Duration
	refreshRequested bool
	fields           map[*SecretField]*managedSecretField
}

type managedSecretField struct {
	validator SecretValidator
	verified  bool
	secret    string
}

func NewManager(config interface{}) *Manager {
	// TODO: populate secrets from config using resolve.go
	return &Manager{
		secrets: make(map[string]*managedSecret),
	}
}

func (m *Manager) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		m.fetchSecretsLoop(ctx)
		cancel()
	}()

	m.cancel = cancel
}

func (m *Manager) Stop() {
	m.cancel()
	// TODO: wait for output
}

func (m *Manager) setSecretValidation(s *SecretField, validator SecretValidator) {
	m.mtx.RLock()
	secret := m.secrets[s.providerId()]
	m.mtx.RUnlock()
	secret.mtx.Lock()
	secret.fields[s].validator = validator
	secret.mtx.Unlock()
}

func (m *Manager) get(s *SecretField) string {
	if inline, ok := s.provider.(*InlineProvider); ok {
		return inline.secret
	}
	m.mtx.RLock()
	secret := m.secrets[s.providerId()]
	m.mtx.RUnlock()
	secret.mtx.RLock()
	defer secret.mtx.RUnlock()
	return secret.fields[s].secret
}

func (m *Manager) triggerRefresh(s *SecretField) {
	m.mtx.RLock()
	secret := m.secrets[s.providerId()]
	m.mtx.RUnlock()
	secret.mtx.Lock()
	defer secret.mtx.Unlock()
	secret.refreshRequested = true
	secret.fields[s].verified = false
	secret.fields[s].secret = secret.pendingSecret
	m.refreshRequested <- struct{}{}
}

// fetchSecretsLoop is a long-running goroutine that periodically fetches secrets.
func (m *Manager) fetchSecretsLoop(ctx context.Context) {
	timer := time.NewTimer(5 * time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		case <-m.refreshRequested:
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

			refreshNeeded := ms.refreshRequested || time.Since(ms.fetched) > ms.refreshInterval
			timeToRefresh := time.Until(ms.fetched.Add(ms.refreshInterval))

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
		timer.Reset(waitTime)

	}
}

// fetchAndStoreSecret performs a single secret fetch, including retry logic with exponential backoff.
// It is robust against hangs in the underlying provider's FetchSecret method.
func (m *Manager) fetchAndStoreSecret(ctx context.Context, ms *managedSecret) {
	var newSecret string
	var err error
	ms.mtx.Lock()
	provider := ms.provider
	ms.mtx.Unlock()

	backoff := fetchInitialBackoff
	for {
		fetchCtx, cancel := context.WithTimeout(ctx, fetchTimeout)

		newSecret, err = provider.FetchSecret(fetchCtx)
		cancel()

		if err == nil {
			break // Success
		}

		select {
		case <-time.After(backoff):
			backoff = min(fetchMaxBackoff, backoff*2)
		case <-ctx.Done():
			return
		}

	}

	ms.mtx.Lock()
	defer ms.mtx.Unlock()

	ms.pendingSecret = newSecret
	ms.fetched = time.Now()
	ms.fetchInProgress = false
	ms.refreshRequested = false

	for _, field := range ms.fields {
		// If a was not verified before, we can swap it immediately
		if field.verified == false {
			field.secret = newSecret
		}
		field.verified = false
		go m.validateAndStoreField(ctx, ms, field, newSecret)
	}
}

// validateAndStoreField performs validation for a single field, including retry logic.
func (m *Manager) validateAndStoreField(ctx context.Context, ms *managedSecret, field *managedSecretField, pendingSecret string) {
	var isValid bool

	ms.mtx.RLock()
	validator := field.validator
	ms.mtx.RUnlock()

	backoff := validationInitialBackoff
	for i := range validationMaxRetries {

		validateCtx, cancel := context.WithTimeout(ctx, validationTimeout)
		isValid = validator == nil || validator.Validate(validateCtx, pendingSecret)
		cancel()

		if isValid {
			break // Success
		}

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

	field.secret = pendingSecret
	field.verified = true
}

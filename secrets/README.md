# Secret Management

The `secrets` package provides a unified way to handle secrets within configuration files for Prometheus and its ecosystem components. It allows secrets to be specified inline, loaded from files, or fetched from other sources through a pluggable provider mechanism.

## Concepts

The package is built around a few core concepts:

  * `SecretField`: A type used in configuration structs to represent a field that holds a secret. It handles the logic for unmarshaling from different secret sources, and the API for accessing secrets.
  * `Provider`: An interface for fetching secrets from a specific source (e.g., inline string, file on disk). The package comes with built-in providers, and new ones can be registered.
  * `Manager`: A component that discovers all `SecretField` instances within a configuration struct, manages their lifecycle, and handles periodic refreshing of secrets.

## How to Use

Using the `secrets` package involves three main steps: defining your configuration struct, initializing the secret manager, and accessing the secret values.

### 1. Define Your Configuration Struct

In your configuration struct, use the `secrets.SecretField` type for any fields that should contain secrets.

```go
package main

import "github.com/prometheus/common/secrets"

type MyConfig struct {
    APIKey    secrets.SecretField `yaml:"api_key"`
    Password  secrets.SecretField `yaml:"password"`
    // ... other config fields
}
```

### 2. Configure Secrets in YAML

Users can then provide secrets in their YAML configuration file.

For simple secrets, an inline string can be used:

```yaml
api_key: "my_super_secret_api_key"
```

To load a secret from a file, use the `file` provider:

```yaml
password:
  file: /path/to/password.txt
```

### 3. Initialize the Secret Manager

After unmarshaling your configuration file into your struct, you must create a `secrets.Manager` to manage the lifecycle of the secrets. The manager is initialized with a pointer to your configuration struct.

```go
import (
    "context"
    "log"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/common/secrets"
    "go.yaml.in/yaml/v2"
)

func main() {
    // A Prometheus registry is needed to register the secret manager's metrics.
    promRegisterer := prometheus.NewRegistry()

    // Load config from file
    configData := []byte(`
api_key: "my_super_secret_api_key"
password:
  file: /path/to/password.txt
`)
    var cfg MyConfig
    if err := yaml.Unmarshal(configData, &cfg); err != nil {
        log.Fatalf("Error unmarshaling config: %v", err)
    }

    // Create a secret manager. This discovers and manages all SecretFields in cfg.
    // The manager will handle refreshing secrets in the background.
    manager, err := secrets.NewManager(promRegisterer, &cfg)
    if err != nil {
        log.Fatalf("Error creating secret manager: %v", err)
    }
    // Start the manager's background refresh loop.
    manager.Start(context.Background())
    defer manager.Stop()


    // ... your application logic ...

    // Wait for the secrets in cfg to be ready.
    for {
        if ready, err := manager.SecretsReady(&cfg); err != nil {
            log.Fatalf("Error checking secret readiness: %v", err)
        } else if ready {
            break
        }
    }

    // Access the secret value when needed.
    apiKey := cfg.APIKey.Get()
    password := cfg.Password.Get()

    log.Printf("API Key: %s", apiKey)
    log.Printf("Password: %s", password)
}
```

### 4. Accessing Secrets

To get the string value of a secret, simply call the `Get()` method on the `SecretField`.

```go
secretValue := myConfig.APIKey.Get()
```

The manager handles caching and refreshing, so `Get()` will always return the current valid secret.

## Built-in Providers

The `secrets` package comes with two built-in providers:

  * `inline`: For secrets that are specified directly as a string in the configuration file. This is the default if a plain string is provided.
    ```yaml
    api_key: "my_inline_secret"
    ```
  * `file`: For secrets that are loaded from a file on disk.
    ```yaml
    password:
      file:
        path: /etc/prometheus/secrets/password
    ```

## Custom Providers

You can extend the functionality by creating your own custom secret providers. A custom provider must implement the `Provider` interface:

```go
type Provider interface {
    // FetchSecret retrieves the secret value.
    FetchSecret(ctx context.Context) (string, error)

    // Name returns the provider's name (e.g., "inline").
    Name() string
}
```

Once you have implemented the interface, you need to register a factory function for your provider with the global `ProviderRegistry`. This is typically done in an `init()` function.

```go
package myprovider

import (
    "context"
    "github.com/prometheus/common/secrets"
)

type MyCustomProvider struct {
    // ... fields for your provider
}

func (p *MyCustomProvider) FetchSecret(ctx context.Context) (string, error) {
    // ... logic to fetch your secret
}

func (p *MyCustomProvider) Name() string {
    return "my_custom_provider"
}

func init() {
    secrets.Providers.Register(func() secrets.Provider {
        return &MyCustomProvider{}
    })
}
```

## Secret Validation

For secrets that can be rotated (e.g., loaded from a file that gets updated), you can provide an optional validation function. This prevents a broken or partially written secret from being loaded into your application after a rotation. The manager will use the new secret only after your validation function returns `true`.

A common use case is to verify that a new authentication token can successfully access a protected endpoint before it is put into active use. This avoids causing monitoring gaps if, for example, a new bearer token is invalid.

To use this feature, implement the `SecretValidator` interface and attach it to a `SecretField` instance.

Here is an example of a validator that checks if an HTTP endpoint can be reached using the new secret as a bearer token. It performs an `HEAD` request and considers the secret valid if the server responds with any status code other than `401 Unauthorized` or `403 Forbidden`.

```go
import (
    "context"
    "fmt"
    "net/http"

    "github.com/prometheus/common/secrets"
)

// HTTPBearerTokenValidator checks if a secret is a valid bearer token for a given URL.
type HTTPBearerTokenValidator struct {
    EndpointURL string
    client      *http.Client
}

func NewHTTPBearerTokenValidator(url string) *HTTPBearerTokenValidator {
    return &HTTPBearerTokenValidator{
        EndpointURL: url,
        client:      &http.Client{},
    }
}

func (v *HTTPBearerTokenValidator) Validate(ctx context.Context, secret string) bool {
    req, err := http.NewRequestWithContext(ctx, "HEAD", v.EndpointURL, nil)
    if err != nil {
        // Could not create the request, so we cannot validate.
        return false
    }

    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", secret))

    resp, err := v.client.Do(req)
    if err != nil {
        // The request failed, so we cannot consider this valid.
        return false
    }
    defer resp.Body.Close()

    // If the status is Unauthorized or Forbidden, the token is invalid.
    // Any other status code (e.g., 200 OK, 404 Not Found) means the token
    // was accepted for authentication, so we consider it valid for rotation.
    return resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden
}

func (v *HTTPBearerTokenValidator) Settings() secrets.ValidationSettings {
    // Return custom settings or use the default.
    return secrets.DefaultValidationSettings()
}

// In your application code, after unmarshaling the config:
validator := NewHTTPBearerTokenValidator("https://my-protected-api.com/v1/status")
cfg.APIKey.SetSecretValidation(validator)
```

The `ValidationSettings` allow you to configure timeouts, backoff, and retry attempts for the validation logic, making the process resilient to temporary network issues.

## Prometheus Metrics

The `Manager` exposes several Prometheus metrics to monitor the state of the secrets it manages. These metrics are registered with the `prometheus.Registerer` that is passed to `NewManager`.

The following metrics are available, all labeled with `provider` and `secret_id`:

  * `prometheus_remote_secret_last_successful_fetch_seconds`: (Gauge) The Unix timestamp of the last successful secret fetch.
  * `prometheus_remote_secret_state`: (Gauge) Describes the current state of a remotely fetched secret (0=success, 1=stale, 2=error, 3=initializing).
  * `prometheus_remote_secret_fetch_success_total`: (Counter) Total number of successful secret fetches.
  * `prometheus_remote_secret_fetch_failures_total`: (Counter) Total number of failed secret fetches.
  * `prometheus_remote_secret_fetch_duration_seconds`: (Histogram) Duration of secret fetch attempts.
  * `prometheus_remote_secret_validation_failures_total`: (Counter) Total number of failed secret validations.

## Error Handling and Panics

The `secrets` package is designed to be robust, but there is one critical error condition that will cause a panic: using a `SecretField` before the `Manager` has been initialized.

If you call `Get()` or `TriggerRefresh()` on a `SecretField` that has not been discovered by a `Manager`, your program will panic with the message:

```
secret field has not been discovered by a manager; was NewManager(&cfg) called?
```

This is a safeguard to prevent the use of unmanaged and potentially empty secrets. To avoid this panic, ensure that you always create a `Manager` by passing a pointer to your configuration struct to `secrets.NewManager` immediately after you unmarshal your configuration.
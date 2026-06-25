# Secret Management

The `secrets` package provides a unified way to handle secrets within configuration files for Prometheus and its ecosystem components. It allows secrets to be specified inline, loaded from files, or fetched from other sources through a pluggable provider mechanism.

See the rendered [GoDoc here](https://pkg.go.dev/github.com/prometheus/common/secrets) if on GitHub.

## How to Use

Using the `secrets` package involves three main steps: defining your configuration struct, initializing the secret manager, and accessing the secret values. Refer to the [package example GoDoc](https://pkg.go.dev/github.com/prometheus/common/secrets#example-package).


## Built-in Providers

The `secrets` package comes with two built-in providers: `inline` and `file`. For more details, please refer to the [GoDoc](https://pkg.go.dev/github.com/prometheus/common/secrets#pkg-variables).

## Custom Providers

You can extend the functionality by creating your own custom secret providers. For a detailed guide on creating custom providers, please refer to the [GoDoc for the `Provider` and `ProviderConfig` interfaces](https://pkg.go.dev/github.com/prometheus/common/secrets#Provider).


## config.Secret

Moving forward, this package aims to completely replace config.Secret. Since `common/config` is imported by both alertmanager & prometheus, we aim for our types
to play well with config.Secret types. Mainly when marshalling secret fields, they will be censored according to `config.MarshalSecretValue`
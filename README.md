# Common
![circleci](https://circleci.com/gh/prometheus/common/tree/main.svg?style=shield)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/prometheus/common/badge)](https://securityscorecards.dev/viewer/?uri=github.com/prometheus/common)


This repository contains Go libraries that are shared across Prometheus
components and libraries. They are considered internal to Prometheus, without
any stability guarantees for external usage.

* **assets**: Embedding of static assets with gzip support
* **config**: Common configuration structures
* **expfmt**: Decoding and encoding for the exposition format
* **model**: Shared data structures
* **promslog**: A logging wrapper around [log/slog](https://pkg.go.dev/log/slog)
* **route**: A routing wrapper around [httprouter](https://github.com/julienschmidt/httprouter) using `context.Context`
* **server**: Common servers
* **version**: Version information and metrics

## Metric/label name validation scheme

The libraries in this Go module share a notion of metric and label name validation scheme.
There are two different schemes to choose from:
* `model.LegacyValidation` => Metric and label names have to conform to the original Prometheus character requirements
* `model.UTF8Validation` => Metric and label names are only required to be valid UTF-8 strings

The active name validation scheme is normally implicitly controlled via the global variable `model.NameValidationScheme`.
It's used by functions such as `model.IsValidMetricName` and `model.LabelName.IsValid`.
_However_, if building with the _experimental_ build tag `localvalidationscheme`, the `model.NameValidationScheme` global is removed, and the API changes to accept the name validation scheme as an explicit parameter.
`model.NameValidationScheme` is deprecated, and at some point, the API currently controlled by the build tag `localvalidationscheme` becomes standard.
For the time being, the `localvalidationscheme` build tag is experimental and the API enabled by it may change.

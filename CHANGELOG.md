# Changelog

## main / unreleased

### What's Changed

## v0.67.2 / 2025-10-28

## What's Changed
* config: Fix panic in `tlsRoundTripper` when CA file is absent by @ndk in https://github.com/prometheus/common/pull/792
* Cleanup linting issues by @SuperQ in https://github.com/prometheus/common/pull/860

## New Contributors
* @ndk made their first contribution in https://github.com/prometheus/common/pull/792

**Full Changelog**: https://github.com/prometheus/common/compare/v0.67.1...v0.67.2

## v0.67.1 / 2025-10-07

## What's Changed
* Remove VERSION file to avoid Go conflict error in https://github.com/prometheus/common/pull/853

**Full Changelog**: https://github.com/prometheus/common/compare/v0.67.0...v0.67.1

## v0.67.0 / 2025-10-07

## What's Changed
* Create CHANGELOG.md for easier communication of library changes, especially possible breaking changes. by @ywwg in https://github.com/prometheus/common/pull/833
* model: New test for validation with dots by @m1k1o in https://github.com/prometheus/common/pull/759
* expfmt: document NewTextParser as required by @burgerdev in https://github.com/prometheus/common/pull/842
* expfmt: Add support for float histograms and gauge histograms by @beorn7 in https://github.com/prometheus/common/pull/843
* Updated minimum Go version to 1.24.0, updated Go dependecies by @SuperQ in https://github.com/prometheus/common/pull/849

## New Contributors
* @m1k1o made their first contribution in https://github.com/prometheus/common/pull/759
* @burgerdev made their first contribution in https://github.com/prometheus/common/pull/842

**Full Changelog**: https://github.com/prometheus/common/compare/v0.66.1...v0.67.0

## v0.66.1 / 2025-09-05

This release has no functional changes, it just drops the dependencies `github.com/grafana/regexp` and `go.uber.org/atomic` and replaces `gopkg.in/yaml.v2` with `go.yaml.in/yaml/v2` (a drop-in replacement).

### What's Changed
* Revert "Use github.com/grafana/regexp instead of regexp" by @aknuds1 in https://github.com/prometheus/common/pull/835
* Move to supported version of yaml parser by @dims in https://github.com/prometheus/common/pull/834
* Revert "Use go.uber.org/atomic instead of sync/atomic (#825)" by @aknuds1 in https://github.com/prometheus/common/pull/838

**Full Changelog**: https://github.com/prometheus/common/compare/v1.20.99...v0.66.1

## v0.66.0 / 2025-09-02

### ⚠️ Breaking Changes ⚠️

* A default-constructed TextParser will be invalid. It must have a valid `scheme` set, so users should use the NewTextParser function to create a valid TextParser. Otherwise parsing will panic with "Invalid name validation scheme requested: unset".

### What's Changed
* model: add constants for type and unit labels. by @bwplotka in https://github.com/prometheus/common/pull/801
* model.ValidationScheme: Support encoding as YAML by @aknuds1 in https://github.com/prometheus/common/pull/799
* fix(promslog): always print time.Duration values as go duration strings by @tjhop in https://github.com/prometheus/common/pull/798
* Add `ValidationScheme` methods `IsValidMetricName` and `IsValidLabelName` by @aknuds1 in https://github.com/prometheus/common/pull/806
* Fix delimited proto not escaped correctly by @thampiotr in https://github.com/prometheus/common/pull/809
* Decoder: Remove use of global name validation and add validation by @ywwg in https://github.com/prometheus/common/pull/808
* ValidationScheme implements pflag.Value and json.Marshaler/Unmarshaler interfaces by @juliusmh in https://github.com/prometheus/common/pull/807
* expfmt: Add NewTextParser function by @aknuds1 in https://github.com/prometheus/common/pull/816

* Enable the godot linter by @aknuds1 in https://github.com/prometheus/common/pull/821
* Enable usestdlibvars linter by @aknuds1 in https://github.com/prometheus/common/pull/820
* Enable unconvert linter by @aknuds1 in https://github.com/prometheus/common/pull/819
* Enable the fatcontext linter by @aknuds1 in https://github.com/prometheus/common/pull/822
* Enable gocritic linter by @aknuds1 in https://github.com/prometheus/common/pull/818
* Use go.uber.org/atomic instead of sync/atomic by @aknuds1 in https://github.com/prometheus/common/pull/825
* Enable revive rule unused-parameter by @aknuds1 in https://github.com/prometheus/common/pull/824
* Enable revive rules by @aknuds1 in https://github.com/prometheus/common/pull/823
* Synchronize common files from prometheus/prometheus by @prombot in https://github.com/prometheus/common/pull/802
* Synchronize common files from prometheus/prometheus by @prombot in https://github.com/prometheus/common/pull/803
* Sync .golangci.yml with prometheus/prometheus by @aknuds1 in https://github.com/prometheus/common/pull/817
* ci: update upload-actions by @ywwg in https://github.com/prometheus/common/pull/814
* docs: fix typo in expfmt.Negotiate by @wmcram in https://github.com/prometheus/common/pull/813
* build(deps): bump golang.org/x/net from 0.40.0 to 0.41.0 by @dependabot[bot] in https://github.com/prometheus/common/pull/800
* build(deps): bump golang.org/x/net from 0.41.0 to 0.42.0 by @dependabot[bot] in https://github.com/prometheus/common/pull/810
* build(deps): bump github.com/stretchr/testify from 1.10.0 to 1.11.1 in /assets by @dependabot[bot] in https://github.com/prometheus/common/pull/826
* build(deps): bump google.golang.org/protobuf from 1.36.6 to 1.36.8 by @dependabot[bot] in https://github.com/prometheus/common/pull/830
* build(deps): bump golang.org/x/net from 0.42.0 to 0.43.0 by @dependabot[bot] in https://github.com/prometheus/common/pull/829
* build(deps): bump github.com/stretchr/testify from 1.10.0 to 1.11.1 by @dependabot[bot] in https://github.com/prometheus/common/pull/827

### New Contributors
* @aknuds1 made their first contribution in https://github.com/prometheus/common/pull/799
* @thampiotr made their first contribution in https://github.com/prometheus/common/pull/809
* @wmcram made their first contribution in https://github.com/prometheus/common/pull/813
* @juliusmh made their first contribution in https://github.com/prometheus/common/pull/807


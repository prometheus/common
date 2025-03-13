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

// otlptranslator is a dependency free package that contains the logic for translating information, such as metric name, unit and type,
// from OpenTelemetry metrics to valid Prometheus metric and label names.
//
// Use BuildCompliantMetricName to build a metric name that complies with traditional Prometheus naming conventions.
// Such conventions exist from a time when Prometheus didn't have support for full UTF-8 characters in metric names.
// For more details see: https://prometheus.io/docs/practices/naming/
//
// Use BuildMetricName to build a metric name that will be accepted by Prometheus with full UTF-8 support.
//
// Use NormalizeLabel to normalize a label name to a valid format that can be used in Prometheus before UTF-8 characters were supported.
package otlptranslator

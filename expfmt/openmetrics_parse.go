// Copyright 2021 The Prometheus Authors
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

package expfmt

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"
	dto "github.com/prometheus/client_model/go"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/pkg/textparse"
)

// OpenMetricsParser is used to parse openmetrics text format.
// zero value is ready to use.
type OpenMetricsParser struct {
	metricFamiliesByName map[string]*dto.MetricFamily
	reader               *bufio.Reader    // Where the parsed input is read through.
	buffer               *bytes.Buffer    // Input is read into buffer before parsing.
	p                    textparse.Parser // The underline parser for parsing openmetrics text format.
	// Key is created with LabelsToSignature.
	summaries map[uint64]*dto.Metric
	// Key is created with LabelsToSignature.
	histograms map[uint64]*dto.Metric
}

// OpenMetricsToMetricFamilies reads 'in' as the openmetrics text exchange format and
// creates MetricFamily proto messages. It returns the MetricFamily proto
// messages in a map where the metric names are the keys, along with any
// error encountered.
//
// If the input contains duplicate metrics (i.e. lines with the same metric name
// and exactly the same label set), the resulting MetricFamily will contain
// duplicate Metric proto messages. Similar is true for duplicate label
// names. Checks for duplicates have to be performed separately, if required.
// Also note that neither the metrics within each MetricFamily are sorted nor
// the label pairs within each Metric.
//
// - Can deal with Counter, Gauge, Histogram, Summary, Untyped metrics types.
//
// - No supported for the following (optional) features: `# UNIT` line, `_created`
//   info type, stateset type, gaugehistogram type which defined at
//   https://github.com/OpenObservability/OpenMetrics/blob/main/specification/OpenMetrics.md#metric-types
//
// - No supported for exemplar.
//
// This method must not be called concurrently. If you want to parse different
// input concurrently, instantiate a separate Parser for each goroutine.
func (p *OpenMetricsParser) OpenMetricsToMetricFamilies(in io.Reader) (map[string]*dto.MetricFamily, error) {
	if err := p.reset(in); err != nil {
		return nil, fmt.Errorf("reset error:%v", err)
	}
	var currentMF *dto.MetricFamily
	var currentMetric *dto.Metric
	for {
		e, err := p.p.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse error:%v", err)
		}
		switch e {
		case textparse.EntryInvalid:
			continue
		case textparse.EntryType:
			n, t := p.p.Type()
			if currentMF = p.metricFamiliesByName[string(n)]; currentMF != nil && currentMF.Type != nil {
				return nil, fmt.Errorf("second TYPE line for metric name %q, or TYPE reported after samples", currentMF.GetName())
			}
			if currentMF == nil {
				currentMF = &dto.MetricFamily{Name: proto.String(string(n))}
				p.metricFamiliesByName[string(n)] = currentMF
			}
			var metricType dto.MetricType
			if currentMF.Type == nil {
				switch strings.ToUpper(string(t)) {
				case "COUNTER":
					metricType = dto.MetricType_COUNTER
					// Complete counter type metric name with the suffix '_total'.
					if !strings.HasSuffix(currentMF.GetName(), "_total") {
						currentMF.Name = proto.String(currentMF.GetName() + "_total")
					}
				case "GAUGE":
					metricType = dto.MetricType_GAUGE
				case "UNKNOWN":
					metricType = dto.MetricType_UNTYPED
				case "SUMMARY":
					metricType = dto.MetricType_SUMMARY
				case "HISTOGRAM":
					metricType = dto.MetricType_HISTOGRAM
				default:
					return nil, fmt.Errorf("unknow metric type %q", t)
				}
				currentMF.Type = metricType.Enum()
			}
		case textparse.EntryHelp:
			n, h := p.p.Help()
			if currentMF = p.metricFamiliesByName[string(n)]; currentMF != nil && currentMF.Help != nil {
				return nil, fmt.Errorf("second TYPE line for metric name %q", currentMF.GetName())
			}
			if currentMF == nil {
				currentMF = &dto.MetricFamily{Name: proto.String(string(n)), Help: proto.String(string(h))}
				p.metricFamiliesByName[string(n)] = currentMF
			}
			if currentMF.Help == nil {
				currentMF.Help = proto.String(string(h))
			}
		case textparse.EntrySeries:
			currentIsSummaryCount := false
			currentIsSummarySum := false
			currentIsHistogramCount := false
			currentIsHistogramSum := false
			lbs := labels.Labels{}
			p.p.Metric(&lbs)
			name := lbs.Get(model.MetricNameLabel)
			m := make(map[string]struct{})
			for _, lb := range lbs {
				if _, exists := m[lb.Name]; exists {
					return nil, fmt.Errorf("metric %q has duplicate label name", name)
				}
				m[lb.Name] = struct{}{}
			}
			_, ts, v := p.p.Series()
			counterName := openMetricsCounterName(name)
			summaryName := openMetricsSummaryName(name)
			histogramName := openMetricsHistogramName(name)
			if currentMF = p.metricFamiliesByName[name]; currentMF != nil {
				// Nothing to do.
			} else if currentMF = p.metricFamiliesByName[counterName]; currentMF != nil && currentMF.GetType() == dto.MetricType_COUNTER {
				// Nothing to do.
			} else if currentMF = p.metricFamiliesByName[summaryName]; currentMF != nil && currentMF.GetType() == dto.MetricType_SUMMARY {
				if openMetricsIsCount(name) {
					currentIsSummaryCount = true
				}
				if openMetricsIsSum(name) {
					currentIsSummarySum = true
				}
			} else if currentMF = p.metricFamiliesByName[histogramName]; currentMF != nil && currentMF.GetType() == dto.MetricType_HISTOGRAM {
				if openMetricsIsCount(name) {
					currentIsHistogramCount = true
				}
				if openMetricsIsSum(name) {
					currentIsHistogramSum = true
				}
			} else {
				currentMF = &dto.MetricFamily{Name: proto.String(name), Type: dto.MetricType_UNTYPED.Enum()}
				p.metricFamiliesByName[name] = currentMF
			}
			currentMetric = &dto.Metric{}
			currentMetricType := currentMF.GetType()
			switch currentMetricType {
			case dto.MetricType_COUNTER, dto.MetricType_GAUGE, dto.MetricType_UNTYPED:
				currentMF.Metric = append(currentMF.Metric, currentMetric)
				switch currentMetricType {
				case dto.MetricType_COUNTER:
					currentMetric.Counter = &dto.Counter{Value: proto.Float64(v)}
				case dto.MetricType_GAUGE:
					currentMetric.Gauge = &dto.Gauge{Value: proto.Float64(v)}
				case dto.MetricType_UNTYPED:
					currentMetric.Untyped = &dto.Untyped{Value: proto.Float64(v)}
				}
				for _, lb := range lbs {
					if lb.Name != model.MetricNameLabel {
						currentMetric.Label = append(currentMetric.Label,
							&dto.LabelPair{Name: proto.String(lb.Name), Value: proto.String(lb.Value)})
					}
				}
				if ts != nil {
					currentMetric.TimestampMs = proto.Int64(*ts)
				}
			case dto.MetricType_SUMMARY:
				m := map[string]string{}
				for _, lb := range lbs {
					if !(lb.Name == model.MetricNameLabel || lb.Name == model.QuantileLabel) {
						m[lb.Name] = lb.Value
					}
				}
				m[model.MetricNameLabel] = *currentMF.Name
				signature := model.LabelsToSignature(m)
				if summary := p.summaries[signature]; summary != nil {
					currentMetric = summary
				} else {
					p.summaries[signature] = currentMetric
					currentMF.Metric = append(currentMF.Metric, currentMetric)
					delete(m, model.MetricNameLabel)
					lbs := labels.FromMap(m)
					for _, lb := range lbs {
						currentMetric.Label = append(currentMetric.Label,
							&dto.LabelPair{Name: proto.String(lb.Name), Value: proto.String(lb.Value)})
					}
				}
				if currentMetric.Summary == nil {
					currentMetric.Summary = &dto.Summary{}
				}
				if currentIsSummaryCount {
					currentMetric.Summary.SampleCount = proto.Uint64(uint64(v))
				}
				if currentIsSummarySum {
					currentMetric.Summary.SampleSum = proto.Float64(v)
				}
				if qs := lbs.Get(model.QuantileLabel); qs != "" {
					quantile, err := openMetricsParseFloat(qs)
					if err != nil {
						return nil, fmt.Errorf("exepected float as quantile got:%q", qs)
					}
					currentMetric.Summary.Quantile = append(currentMetric.Summary.Quantile,
						&dto.Quantile{Quantile: proto.Float64(quantile), Value: proto.Float64(v)})
				}
				if ts != nil {
					currentMetric.TimestampMs = proto.Int64(*ts)
				}
			case dto.MetricType_HISTOGRAM:
				m := map[string]string{}
				for _, lb := range lbs {
					if !(lb.Name == model.MetricNameLabel || lb.Name == model.BucketLabel) {
						m[lb.Name] = lb.Value
					}
				}
				m[model.MetricNameLabel] = *currentMF.Name
				signature := model.LabelsToSignature(m)
				if histogram := p.histograms[signature]; histogram != nil {
					currentMetric = histogram
				} else {
					p.histograms[signature] = currentMetric
					currentMF.Metric = append(currentMF.Metric, currentMetric)
					delete(m, model.MetricNameLabel)
					lbs := labels.FromMap(m)
					for _, lb := range lbs {
						currentMetric.Label = append(currentMetric.Label,
							&dto.LabelPair{Name: proto.String(lb.Name), Value: proto.String(lb.Value)})
					}
				}
				if currentMetric.Histogram == nil {
					currentMetric.Histogram = &dto.Histogram{}
				}
				if currentIsHistogramCount {
					currentMetric.Histogram.SampleCount = proto.Uint64(uint64(v))
				}
				if currentIsHistogramSum {
					currentMetric.Histogram.SampleSum = proto.Float64(v)
				}
				if bs := lbs.Get(model.BucketLabel); bs != "" {
					bucket, err := openMetricsParseFloat(bs)
					if err != nil {
						return nil, fmt.Errorf("expected float as bucket bound got:%q", bs)
					}
					currentMetric.Histogram.Bucket = append(currentMetric.Histogram.Bucket,
						&dto.Bucket{UpperBound: proto.Float64(bucket), CumulativeCount: proto.Uint64(uint64(v))})
				}
				if ts != nil {
					currentMetric.TimestampMs = proto.Int64(*ts)
				}
			}
		case textparse.EntryComment:
			continue
		case textparse.EntryUnit:
			continue
		}
	}
	for k, mf := range p.metricFamiliesByName {
		if len(mf.GetMetric()) == 0 {
			delete(p.metricFamiliesByName, k)
		}
	}
	return p.metricFamiliesByName, nil
}

func (p *OpenMetricsParser) reset(in io.Reader) error {
	if p.buffer == nil {
		p.buffer = bytes.NewBuffer(nil)
	} else {
		p.buffer.Reset()
	}
	if p.reader == nil {
		p.reader = bufio.NewReader(in)
	} else {
		p.reader.Reset(in)
	}
	if _, err := io.Copy(p.buffer, p.reader); err != nil {
		return err
	}
	p.p = textparse.NewOpenMetricsParser(p.buffer.Bytes())
	p.metricFamiliesByName = map[string]*dto.MetricFamily{}
	if p.summaries == nil {
		p.summaries = map[uint64]*dto.Metric{}
	} else {
		for k := range p.summaries {
			delete(p.summaries, k)
		}
	}
	if p.histograms == nil {
		p.histograms = map[uint64]*dto.Metric{}
	} else {
		for k := range p.histograms {
			delete(p.histograms, k)
		}
	}
	return nil
}

func openMetricsCounterName(name string) string {
	if len(name) > len("_total") && strings.HasSuffix(name, "_total") {
		return name[:len(name)-6]
	}
	return name
}

func openMetricsIsCount(name string) bool {
	return len(name) > 6 && name[len(name)-6:] == "_count"
}

func openMetricsIsSum(name string) bool {
	return len(name) > 4 && name[len(name)-4:] == "_sum"
}

func openMetricsIsBucket(name string) bool {
	return len(name) > 7 && name[len(name)-7:] == "_bucket"
}

func openMetricsSummaryName(name string) string {
	switch {
	case openMetricsIsCount(name):
		return name[:len(name)-6]
	case openMetricsIsSum(name):
		return name[:len(name)-4]
	default:
		return name
	}
}

func openMetricsHistogramName(name string) string {
	switch {
	case openMetricsIsCount(name):
		return name[:len(name)-6]
	case openMetricsIsSum(name):
		return name[:len(name)-4]
	case openMetricsIsBucket(name):
		return name[:len(name)-7]
	default:
		return name
	}
}

func openMetricsParseFloat(s string) (float64, error) {
	if strings.ContainsAny(s, "pP_") {
		return 0, fmt.Errorf("unsupported character in float")
	}
	return strconv.ParseFloat(s, 64)
}

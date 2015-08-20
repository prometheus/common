// Copyright 2015 The Prometheus Authors
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
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"

	dto "github.com/prometheus/client_model/go"

	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	"github.com/prometheus/common/expfmt/text"
	"github.com/prometheus/common/model"
)

// Decoder types decode an input stream into metric families.
type Decoder interface {
	Decode(*dto.MetricFamily) error
}

type DecodeOptions struct {
	// Timestamp is added to each value from the stream that has no explicit timestamp set.
	Timestamp model.Timestamp
}

// NewDecor returns a new decoder based on the HTTP header.
func NewDecoder(r io.Reader, h http.Header) (Decoder, error) {
	ct := h.Get(hdrContentType)

	mediatype, params, err := mime.ParseMediaType(ct)
	if err != nil {
		return nil, fmt.Errorf("invalid Content-Type header %q: %s", ct, err)
	}

	const (
		protoType = ProtoType + "/" + ProtoSubType
		textType  = "text/plain"
	)

	switch mediatype {
	case protoType:
		if p := params["proto"]; p != ProtoProtocol {
			return nil, fmt.Errorf("unrecognized protocol message %s", p)
		}
		if e := params["encoding"]; e != "delimited" {
			return nil, fmt.Errorf("unsupported encoding %s", e)
		}
		return &protoDecoder{r: r}, nil

	case textType:
		if v, ok := params["version"]; ok && v != "0.0.4" {
			return nil, fmt.Errorf("unrecognized protocol version %s", v)
		}
		return &textDecoder{r: r}, nil

	default:
		return nil, fmt.Errorf("unsupported media type %q, expected %q or %q", mediatype, protoType, textType)
	}
}

// protoDecoder implements the Decoder interface for protocol buffers.
type protoDecoder struct {
	r io.Reader
}

// Decode implements the Decoder interface.
func (d *protoDecoder) Decode(v *dto.MetricFamily) error {
	_, err := pbutil.ReadDelimited(d.r, v)
	return err
}

// textDecoder implements the Decoder interface for the text protcol.
type textDecoder struct {
	r    io.Reader
	p    text.Parser
	fams []*dto.MetricFamily
}

// Decode implements the Decoder interface.
func (d *textDecoder) Decode(v *dto.MetricFamily) error {
	// TODO(fabxc): Wrap this as a line reader to make streaming safer.
	if len(d.fams) == 0 {
		// No cached metric families, read everything and parse metrics.
		fams, err := d.p.TextToMetricFamilies(d.r)
		if err != nil {
			return err
		}
		if len(fams) == 0 {
			return io.EOF
		}
		for _, f := range fams {
			d.fams = append(d.fams, f)
		}
	}
	*v = *d.fams[len(d.fams)-1]
	d.fams = d.fams[:len(d.fams)-1]
	return nil
}

type SampleDecoder struct {
	Dec  Decoder
	f    dto.MetricFamily
	Opts *DecodeOptions
}

func (sd *SampleDecoder) Decode(s *model.Samples) error {
	if err := sd.Dec.Decode(&sd.f); err != nil {
		return err
	}
	*s = extractSamples(&sd.f, sd.Opts)
	return nil
}

// Extract samples builds a slice of samples from the provided metric families.
func ExtractSamples(o *DecodeOptions, fams ...*dto.MetricFamily) model.Samples {
	var all model.Samples
	for _, f := range fams {
		all = append(all, extractSamples(f, o)...)
	}
	return all
}

func extractSamples(f *dto.MetricFamily, o *DecodeOptions) model.Samples {
	switch *f.Type {
	case dto.MetricType_COUNTER:
		return extractCounter(o, f)
	case dto.MetricType_GAUGE:
		return extractGauge(o, f)
	case dto.MetricType_SUMMARY:
		return extractSummary(o, f)
	case dto.MetricType_UNTYPED:
		return extractUntyped(o, f)
	case dto.MetricType_HISTOGRAM:
		return extractHistogram(o, f)
	}
	panic("expfmt.extractSamples: unknown metric family type")
}

func extractCounter(o *DecodeOptions, f *dto.MetricFamily) model.Samples {
	samples := make(model.Samples, 0, len(f.Metric))

	for _, m := range f.Metric {
		if m.Counter == nil {
			continue
		}

		sample := &model.Sample{
			Metric: model.Metric{},
			Value:  model.SampleValue(m.Counter.GetValue()),
		}
		samples = append(samples, sample)

		if m.TimestampMs != nil {
			sample.Timestamp = model.TimestampFromUnixNano(*m.TimestampMs * 1000000)
		} else {
			sample.Timestamp = o.Timestamp
		}

		metric := sample.Metric
		for _, p := range m.Label {
			metric[model.LabelName(p.GetName())] = model.LabelValue(p.GetValue())
		}
		metric[model.MetricNameLabel] = model.LabelValue(f.GetName())
	}

	return samples
}

func extractGauge(o *DecodeOptions, f *dto.MetricFamily) model.Samples {
	samples := make(model.Samples, 0, len(f.Metric))

	for _, m := range f.Metric {
		if m.Gauge == nil {
			continue
		}

		sample := &model.Sample{
			Metric: model.Metric{},
			Value:  model.SampleValue(m.Gauge.GetValue()),
		}
		samples = append(samples, sample)

		if m.TimestampMs != nil {
			sample.Timestamp = model.TimestampFromUnixNano(*m.TimestampMs * 1000000)
		} else {
			sample.Timestamp = o.Timestamp
		}

		metric := sample.Metric
		for _, p := range m.Label {
			metric[model.LabelName(p.GetName())] = model.LabelValue(p.GetValue())
		}
		metric[model.MetricNameLabel] = model.LabelValue(f.GetName())
	}

	return samples
}

func extractUntyped(o *DecodeOptions, f *dto.MetricFamily) model.Samples {
	samples := make(model.Samples, 0, len(f.Metric))

	for _, m := range f.Metric {
		if m.Untyped == nil {
			continue
		}

		sample := &model.Sample{
			Metric: model.Metric{},
			Value:  model.SampleValue(m.Untyped.GetValue()),
		}
		samples = append(samples, sample)

		if m.TimestampMs != nil {
			sample.Timestamp = model.TimestampFromUnixNano(*m.TimestampMs * 1000000)
		} else {
			sample.Timestamp = o.Timestamp
		}

		metric := sample.Metric
		for _, p := range m.Label {
			metric[model.LabelName(p.GetName())] = model.LabelValue(p.GetValue())
		}
		metric[model.MetricNameLabel] = model.LabelValue(f.GetName())
	}

	return samples
}

func extractSummary(o *DecodeOptions, f *dto.MetricFamily) model.Samples {
	samples := make(model.Samples, 0, len(f.Metric))

	for _, m := range f.Metric {
		if m.Summary == nil {
			continue
		}

		timestamp := o.Timestamp
		if m.TimestampMs != nil {
			timestamp = model.TimestampFromUnixNano(*m.TimestampMs * 1000000)
		}

		for _, q := range m.Summary.Quantile {
			sample := &model.Sample{
				Metric:    model.Metric{},
				Value:     model.SampleValue(q.GetValue()),
				Timestamp: timestamp,
			}
			samples = append(samples, sample)

			metric := sample.Metric
			for _, p := range m.Label {
				metric[model.LabelName(p.GetName())] = model.LabelValue(p.GetValue())
			}
			// BUG(matt): Update other names to "quantile".
			metric[model.LabelName(model.QuantileLabel)] = model.LabelValue(fmt.Sprint(q.GetQuantile()))
			metric[model.MetricNameLabel] = model.LabelValue(f.GetName())
		}

		if m.Summary.SampleSum != nil {
			sum := &model.Sample{
				Metric:    model.Metric{},
				Value:     model.SampleValue(m.Summary.GetSampleSum()),
				Timestamp: timestamp,
			}
			samples = append(samples, sum)

			metric := sum.Metric
			for _, p := range m.Label {
				metric[model.LabelName(p.GetName())] = model.LabelValue(p.GetValue())
			}
			metric[model.MetricNameLabel] = model.LabelValue(f.GetName() + "_sum")
		}

		if m.Summary.SampleCount != nil {
			count := &model.Sample{
				Metric:    model.Metric{},
				Value:     model.SampleValue(m.Summary.GetSampleCount()),
				Timestamp: timestamp,
			}
			samples = append(samples, count)

			metric := count.Metric
			for _, p := range m.Label {
				metric[model.LabelName(p.GetName())] = model.LabelValue(p.GetValue())
			}
			metric[model.MetricNameLabel] = model.LabelValue(f.GetName() + "_count")
		}
	}

	return samples
}

func extractHistogram(o *DecodeOptions, f *dto.MetricFamily) model.Samples {
	samples := make(model.Samples, 0, len(f.Metric))

	for _, m := range f.Metric {
		if m.Histogram == nil {
			continue
		}

		timestamp := o.Timestamp
		if m.TimestampMs != nil {
			timestamp = model.TimestampFromUnixNano(*m.TimestampMs * 1000000)
		}

		infSeen := false

		for _, q := range m.Histogram.Bucket {
			sample := &model.Sample{
				Metric:    model.Metric{},
				Value:     model.SampleValue(q.GetCumulativeCount()),
				Timestamp: timestamp,
			}
			samples = append(samples, sample)

			metric := sample.Metric
			for _, p := range m.Label {
				metric[model.LabelName(p.GetName())] = model.LabelValue(p.GetValue())
			}
			metric[model.LabelName(model.BucketLabel)] = model.LabelValue(fmt.Sprint(q.GetUpperBound()))
			metric[model.MetricNameLabel] = model.LabelValue(f.GetName() + "_bucket")

			if math.IsInf(q.GetUpperBound(), +1) {
				infSeen = true
			}
		}

		if m.Histogram.SampleSum != nil {
			sum := &model.Sample{
				Metric:    model.Metric{},
				Value:     model.SampleValue(m.Histogram.GetSampleSum()),
				Timestamp: timestamp,
			}
			samples = append(samples, sum)

			metric := sum.Metric
			for _, p := range m.Label {
				metric[model.LabelName(p.GetName())] = model.LabelValue(p.GetValue())
			}
			metric[model.MetricNameLabel] = model.LabelValue(f.GetName() + "_sum")
		}

		if m.Histogram.SampleCount != nil {
			count := &model.Sample{
				Metric:    model.Metric{},
				Value:     model.SampleValue(m.Histogram.GetSampleCount()),
				Timestamp: timestamp,
			}
			samples = append(samples, count)

			metric := count.Metric
			for _, p := range m.Label {
				metric[model.LabelName(p.GetName())] = model.LabelValue(p.GetValue())
			}
			metric[model.MetricNameLabel] = model.LabelValue(f.GetName() + "_count")

			if !infSeen {
				infBucket := &model.Sample{
					Metric:    model.Metric{},
					Value:     count.Value,
					Timestamp: timestamp,
				}
				samples = append(samples, infBucket)

				metric := infBucket.Metric
				for _, p := range m.Label {
					metric[model.LabelName(p.GetName())] = model.LabelValue(p.GetValue())
				}
				metric[model.LabelName(model.BucketLabel)] = model.LabelValue("+Inf")
				metric[model.MetricNameLabel] = model.LabelValue(f.GetName() + "_bucket")
			}
		}
	}

	return samples
}

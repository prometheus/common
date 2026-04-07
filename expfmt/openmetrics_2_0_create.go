// Copyright The Prometheus Authors
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
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"

	dto "github.com/prometheus/client_model/go"
)

// MetricFamilyToOpenMetrics20 converts a MetricFamily proto message into the
// OpenMetrics text format version 2.0.0 and writes the resulting lines to 'out'.
// It returns the number of bytes written and any error encountered.
func MetricFamilyToOpenMetrics20(out io.Writer, in *dto.MetricFamily, options ...EncoderOption) (written int, err error) {
	_ = options
	name := in.GetName()
	if name == "" {
		return 0, fmt.Errorf("MetricFamily has no name: %s", in)
	}

	// Try the interface upgrade. If it doesn't work, we'll use a
	// bufio.Writer from the sync.Pool.
	w, ok := out.(enhancedWriter)
	if !ok {
		b := bufPool.Get().(*bufio.Writer)
		b.Reset(out)
		w = b
		defer func() {
			bErr := b.Flush()
			if err == nil {
				err = bErr
			}
			bufPool.Put(b)
		}()
	}

	var (
		n          int
		metricType = in.GetType()
	)

	// Comments, first HELP, then TYPE.
	if in.Help != nil {
		n, err = w.WriteString("# HELP ")
		written += n
		if err != nil {
			return written, err
		}
		n, err = writeName(w, name)
		written += n
		if err != nil {
			return written, err
		}
		err = w.WriteByte(' ')
		written++
		if err != nil {
			return written, err
		}
		n, err = writeEscapedString(w, *in.Help, true)
		written += n
		if err != nil {
			return written, err
		}
		err = w.WriteByte('\n')
		written++
		if err != nil {
			return written, err
		}
	}
	n, err = w.WriteString("# TYPE ")
	written += n
	if err != nil {
		return written, err
	}
	n, err = writeName(w, name)
	written += n
	if err != nil {
		return written, err
	}
	switch metricType {
	case dto.MetricType_COUNTER:
		n, err = w.WriteString(" counter\n")
	case dto.MetricType_GAUGE:
		n, err = w.WriteString(" gauge\n")
	case dto.MetricType_SUMMARY:
		n, err = w.WriteString(" summary\n")
	case dto.MetricType_UNTYPED:
		n, err = w.WriteString(" unknown\n")
	case dto.MetricType_HISTOGRAM:
		n, err = w.WriteString(" histogram\n")
	case dto.MetricType_GAUGE_HISTOGRAM:
		n, err = w.WriteString(" gaugehistogram\n")
	default:
		return written, fmt.Errorf("unknown metric type %s", metricType.String())
	}
	written += n
	if err != nil {
		return written, err
	}
	if in.Unit != nil {
		n, err = w.WriteString("# UNIT ")
		written += n
		if err != nil {
			return written, err
		}
		n, err = writeName(w, name)
		written += n
		if err != nil {
			return written, err
		}

		err = w.WriteByte(' ')
		written++
		if err != nil {
			return written, err
		}
		n, err = writeEscapedString(w, *in.Unit, true)
		written += n
		if err != nil {
			return written, err
		}
		err = w.WriteByte('\n')
		written++
		if err != nil {
			return written, err
		}
	}

	// Finally the samples, one line for each.
	for _, metric := range in.Metric {
		switch metricType {
		case dto.MetricType_COUNTER:
			if metric.Counter == nil {
				return written, fmt.Errorf("expected counter in metric %s %s", name, metric)
			}
			n, err = writeOpenMetrics20Sample(w, name, metric, metric.Counter.GetValue(), 0, false, metric.Counter.Exemplar)
		case dto.MetricType_GAUGE:
			if metric.Gauge == nil {
				return written, fmt.Errorf("expected gauge in metric %s %s", name, metric)
			}
			n, err = writeOpenMetrics20Sample(w, name, metric, metric.Gauge.GetValue(), 0, false, nil)
		case dto.MetricType_UNTYPED:
			if metric.Untyped == nil {
				return written, fmt.Errorf("expected untyped in metric %s %s", name, metric)
			}
			n, err = writeOpenMetrics20Sample(w, name, metric, metric.Untyped.GetValue(), 0, false, nil)
		case dto.MetricType_SUMMARY:
			if metric.Summary == nil {
				return written, fmt.Errorf("expected summary in metric %s %s", name, metric)
			}
			n, err = writeCompositeSummary(w, name, metric)
		case dto.MetricType_HISTOGRAM, dto.MetricType_GAUGE_HISTOGRAM:
			if metric.Histogram == nil {
				return written, fmt.Errorf("expected histogram in metric %s %s", name, metric)
			}
			n, err = writeCompositeHistogram(w, name, metric, metricType == dto.MetricType_GAUGE_HISTOGRAM)
		default:
			return written, fmt.Errorf("unexpected type in metric %s %s", name, metric)
		}
		written += n
		if err != nil {
			return written, err
		}
	}
	return written, nil
}

// writeOpenMetrics20Sample writes a single sample for simple types (Counter, Gauge, Untyped).
func writeOpenMetrics20Sample(w enhancedWriter, name string, metric *dto.Metric, floatValue float64, intValue uint64, useIntValue bool, exemplar *dto.Exemplar) (int, error) {
	written := 0
	n, err := writeOpenMetricsNameAndLabelPairs(w, name, metric.Label, "", 0)
	written += n
	if err != nil {
		return written, err
	}
	err = w.WriteByte(' ')
	written++
	if err != nil {
		return written, err
	}

	if useIntValue {
		n, err = writeUint(w, intValue)
	} else {
		n, err = writeFloat(w, floatValue)
	}
	written += n
	if err != nil {
		return written, err
	}

	if metric.TimestampMs != nil {
		err = w.WriteByte(' ')
		written++
		if err != nil {
			return written, err
		}
		n, err = writeOpenMetrics20Timestamp(w, float64(*metric.TimestampMs)/1000)
		written += n
		if err != nil {
			return written, err
		}
	}

	// Start Timestamp for Counter
	if metric.Counter != nil && metric.Counter.CreatedTimestamp != nil {
		n, err = w.WriteString(" st@")
		written += n
		if err != nil {
			return written, err
		}
		ts := metric.Counter.CreatedTimestamp
		n, err = writeOpenMetrics20Timestamp(w, float64(ts.GetSeconds())+float64(ts.GetNanos())/1e9)
		written += n
		if err != nil {
			return written, err
		}
	}

	if exemplar != nil && len(exemplar.Label) > 0 {
		n, err = writeExemplar(w, exemplar)
		written += n
		if err != nil {
			return written, err
		}
	}

	err = w.WriteByte('\n')
	written++
	if err != nil {
		return written, err
	}
	return written, nil
}

// writeOpenMetrics20Timestamp writes a float64 as a timestamp without scientific notation.
func writeOpenMetrics20Timestamp(w enhancedWriter, f float64) (int, error) {
	switch {
	case math.IsNaN(f):
		return w.WriteString("NaN")
	case math.IsInf(f, +1):
		return w.WriteString("+Inf")
	case math.IsInf(f, -1):
		return w.WriteString("-Inf")
	default:
		bp := numBufPool.Get().(*[]byte)
		*bp = strconv.AppendFloat((*bp)[:0], f, 'f', -1, 64)
		written, err := w.Write(*bp)
		numBufPool.Put(bp)
		return written, err
	}
}

// Stubs for Summary and Histogram

func writeCompositeSummary(w enhancedWriter, name string, metric *dto.Metric) (int, error) {
	_ = w
	_ = name
	_ = metric
	return 0, errors.New("summary not implemented yet")
}

func writeCompositeHistogram(w enhancedWriter, name string, metric *dto.Metric, isGauge bool) (int, error) {
	_ = w
	_ = name
	_ = metric
	_ = isGauge
	return 0, errors.New("histogram not implemented yet")
}

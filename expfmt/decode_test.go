package expfmt

import (
	"io"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/prometheus/common/model"
)

var (
	ts = model.Now()
	in = `
# Only a quite simple scenario with two metric families.
# More complicated tests of the parser itself can be found in the text package.
# TYPE mf2 counter
mf2 3
mf1{label="value1"} -3.14 123456
mf1{label="value2"} 42
mf2 4
`
	out = model.Samples{
		&model.Sample{
			Metric:    model.Metric{model.MetricNameLabel: "mf1", "label": "value1"},
			Value:     -3.14,
			Timestamp: 123456,
		},
		&model.Sample{
			Metric:    model.Metric{model.MetricNameLabel: "mf1", "label": "value2"},
			Value:     42,
			Timestamp: ts,
		},
		&model.Sample{
			Metric:    model.Metric{model.MetricNameLabel: "mf2"},
			Value:     3,
			Timestamp: ts,
		},
		&model.Sample{
			Metric:    model.Metric{model.MetricNameLabel: "mf2"},
			Value:     4,
			Timestamp: ts,
		},
	}
)

func TestTextDecoder(t *testing.T) {
	dec := &SampleDecoder{
		Dec: &textDecoder{r: strings.NewReader(in)},
		Opts: &DecodeOptions{
			Timestamp: ts,
		},
	}
	var all model.Samples
	for {
		var smpls model.Samples
		err := dec.Decode(&smpls)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, smpls...)
	}
	sort.Sort(all)
	sort.Sort(out)
	if !reflect.DeepEqual(all, out) {
		t.Fatalf("output does not match")
	}
}

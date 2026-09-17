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
package model

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"
)

func generateData(timeseries, datapoints int) (floatMatrix, histogramMatrix Matrix) {
	for i := range timeseries {
		lset := map[LabelName]LabelValue{
			MetricNameLabel: LabelValue("timeseries_" + strconv.Itoa(i)),
			"foo":           "bar",
		}
		now := Time(1677587274055)
		floats := make([]SamplePair, datapoints)
		histograms := make([]SampleHistogramPair, datapoints)

		for x := datapoints; x > 0; x-- {
			f := float64(x)
			floats[x-1] = SamplePair{
				// Set the time back assuming a 15s interval. Since this is used for
				// Marshal/Unmarshal testing the actual interval doesn't matter.
				Timestamp: now.Add(time.Second * -15 * time.Duration(x)),
				Value:     SampleValue(f),
			}
			histograms[x-1] = SampleHistogramPair{
				Timestamp: now.Add(time.Second * -15 * time.Duration(x)),
				Histogram: &SampleHistogram{
					Count: FloatString(13.5 * f),
					Sum:   FloatString(.1 * f),
					Buckets: HistogramBuckets{
						{
							Boundaries: 1,
							Lower:      -4870.992343051145,
							Upper:      -4466.7196729968955,
							Count:      FloatString(1 * f),
						},
						{
							Boundaries: 1,
							Lower:      -861.0779292198035,
							Upper:      -789.6119426088657,
							Count:      FloatString(2 * f),
						},
						{
							Boundaries: 1,
							Lower:      -558.3399591246119,
							Upper:      -512,
							Count:      FloatString(3 * f),
						},
						{
							Boundaries: 0,
							Lower:      2048,
							Upper:      2233.3598364984477,
							Count:      FloatString(1.5 * f),
						},
						{
							Boundaries: 0,
							Lower:      2896.3093757400984,
							Upper:      3158.4477704354626,
							Count:      FloatString(2.5 * f),
						},
						{
							Boundaries: 0,
							Lower:      4466.7196729968955,
							Upper:      4870.992343051145,
							Count:      FloatString(3.5 * f),
						},
					},
				},
			}
		}

		fss := &SampleStream{
			Metric: Metric(lset),
			Values: floats,
		}
		hss := &SampleStream{
			Metric:     Metric(lset),
			Histograms: histograms,
		}

		floatMatrix = append(floatMatrix, fss)
		histogramMatrix = append(histogramMatrix, hss)
	}
	return floatMatrix, histogramMatrix
}

func BenchmarkSamplesJSONUnmarshal(b *testing.B) {
	for _, timeseriesCount := range []int{10, 100, 1000} {
		b.Run("series="+strconv.Itoa(timeseriesCount), func(b *testing.B) {
			for _, datapointCount := range []int{10, 100, 1000} {
				b.Run("dp="+strconv.Itoa(datapointCount), func(b *testing.B) {
					floats, histograms := generateData(timeseriesCount, datapointCount)

					floatBytes, err := json.Marshal(floats)
					if err != nil {
						b.Fatalf("Error marshaling: %v", err)
					}
					histogramBytes, err := json.Marshal(histograms)
					if err != nil {
						b.Fatalf("Error marshaling: %v", err)
					}

					b.Run("type=floats", func(b *testing.B) {
						b.ReportAllocs()
						for b.Loop() {
							var m Matrix
							if err := json.Unmarshal(floatBytes, &m); err != nil {
								b.Fatal(err)
							}
						}
					})
					b.Run("type=histograms", func(b *testing.B) {
						b.ReportAllocs()
						for b.Loop() {
							var m Matrix
							if err := json.Unmarshal(histogramBytes, &m); err != nil {
								b.Fatal(err)
							}
						}
					})
				})
			}
		})
	}
}

// Copyright 2014 The Prometheus Authors
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
	"fmt"
	"maps"
	"runtime"
	"slices"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLabelsToSignature(t *testing.T) {
	scenarios := []struct {
		in  map[string]string
		out uint64
	}{
		{
			in:  map[string]string{},
			out: 14695981039346656037,
		},
		{
			in:  map[string]string{"name": "garland, briggs", "fear": "love is not enough"},
			out: 5799056148416392346,
		},
	}

	for i, scenario := range scenarios {
		actual := LabelsToSignature(scenario.in)

		if actual != scenario.out {
			t.Errorf("%d. expected %d, got %d", i, scenario.out, actual)
		}
	}
}

func TestMetricToFingerprint(t *testing.T) {
	scenarios := []struct {
		in  LabelSet
		out Fingerprint
	}{
		{
			in:  LabelSet{},
			out: 14695981039346656037,
		},
		{
			in:  LabelSet{"name": "garland, briggs", "fear": "love is not enough"},
			out: 5799056148416392346,
		},
	}

	for i, scenario := range scenarios {
		actual := labelSetToFingerprint(scenario.in)

		if actual != scenario.out {
			t.Errorf("%d. expected %d, got %d", i, scenario.out, actual)
		}
	}
}

func TestLabelSetFingerprintForLabels(t *testing.T) {
	ls := LabelSet{"alertname": "HighLatency", "job": "api", "instance": "host-1:9090"}
	wide := LabelSet{}
	wideNames := make([]LabelName, 0, 2*fingerprintNamesBuffer)
	for i := range 2 * fingerprintNamesBuffer {
		name := LabelName(fmt.Sprintf("label_%02d", i))
		wide[name] = LabelValue(strconv.Itoa(i))
		wideNames = append(wideNames, name)
	}
	// In reverse order, with repeated and absent names, so that the heap
	// path sorts and deduplicates too.
	unsortedWideNames := append(slices.Clone(wideNames), "missing", "label_07", "label_42", "other")
	sortedWideNames := slices.Clone(unsortedWideNames)
	slices.Sort(sortedWideNames)
	slices.Reverse(unsortedWideNames)
	// Repeated present names are appended before deduplication, so enough of
	// them grow labelNames past the buffer chosen from the set and name counts.
	repeated := func(names []LabelName, count int) []LabelName {
		out := make([]LabelName, 0, count)
		for i := range count {
			out = append(out, names[i%len(names)])
		}
		return out
	}
	two := LabelSet{"job": "api", "instance": "host-1:9090"}

	for _, scenario := range []struct {
		desc  string
		in    LabelSet
		names []LabelName
	}{
		{desc: "no names", in: ls},
		{desc: "all names", in: ls, names: []LabelName{"alertname", "job", "instance"}},
		{desc: "sorted subset", in: ls, names: []LabelName{"alertname", "job"}},
		{desc: "unsorted subset", in: ls, names: []LabelName{"job", "alertname"}},
		{desc: "absent names", in: ls, names: []LabelName{"missing", "job", "other"}},
		{desc: "only absent names", in: ls, names: []LabelName{"missing"}},
		{desc: "sorted repeated names", in: ls, names: []LabelName{"instance", "instance", "job"}},
		{desc: "repeated names", in: ls, names: []LabelName{"job", "job", "instance"}},
		{desc: "repeated names past the small buffer", in: two, names: repeated([]LabelName{"job", "instance"}, fingerprintNamesSmallBuffer+1)},
		{desc: "repeated names past the stack buffer", in: ls, names: repeated([]LabelName{"job", "alertname", "instance"}, fingerprintNamesBuffer+8)},
		{desc: "empty set", in: LabelSet{}, names: []LabelName{"job"}},
		{desc: "small subset of a wide set", in: wide, names: []LabelName{"label_40", "label_03"}},
		{desc: "more names than the stack buffer", in: wide, names: wideNames},
		{desc: "sorted, repeated and absent names, more than the stack buffer", in: wide, names: sortedWideNames},
		{desc: "unsorted, repeated and absent names, more than the stack buffer", in: wide, names: unsortedWideNames},
		{desc: "empty label value", in: LabelSet{"job": "", "instance": "host-1:9090"}, names: []LabelName{"job", "missing"}},
	} {
		t.Run(scenario.desc, func(t *testing.T) {
			subset := LabelSet{}
			subsetMap := map[string]string{}
			for _, name := range scenario.names {
				if value, ok := scenario.in[name]; ok {
					subset[name] = value
					subsetMap[string(name)] = string(value)
				}
			}
			names := slices.Clone(scenario.names)
			got := scenario.in.FingerprintForLabels(names...)
			require.Equalf(t, scenario.names, names, "names must not be modified")
			require.Equal(t, subset.Fingerprint(), got)
			// LabelsToSignature is an independent implementation.
			require.Equal(t, Fingerprint(LabelsToSignature(subsetMap)), got)
		})
	}
}

func TestFingerprintAllocations(t *testing.T) {
	ls := LabelSet{"alertname": "HighLatency", "job": "api", "instance": "host-1:9090"}
	sized := func(n int) LabelSet {
		ls := LabelSet{}
		for i := range n {
			ls[LabelName("label_"+strconv.Itoa(i))] = LabelValue(strconv.Itoa(i))
		}
		return ls
	}
	atBuffer, overBuffer := sized(fingerprintNamesBuffer), sized(fingerprintNamesBuffer+1)
	allNames := slices.Sorted(maps.Keys(overBuffer))
	mostlyAbsent := []LabelName{"label_3", "label_1"}
	for i := range 2 * fingerprintNamesBuffer {
		mostlyAbsent = append(mostlyAbsent, LabelName("missing_"+strconv.Itoa(i)))
	}

	require.Zero(t, testing.AllocsPerRun(100, func() { ls.Fingerprint() }))
	require.Zero(t, testing.AllocsPerRun(100, func() { ls.FingerprintForLabels("alertname", "job", "job", "missing") }))
	require.Zero(t, testing.AllocsPerRun(100, func() { ls.FingerprintForLabels("job", "job", "alertname", "missing") }))
	for _, n := range []int{1, fingerprintNamesSmallBuffer, fingerprintNamesSmallBuffer + 1} {
		small := sized(n)
		require.Zero(t, testing.AllocsPerRun(100, func() { small.Fingerprint() }))
	}
	require.Zero(t, testing.AllocsPerRun(100, func() { atBuffer.Fingerprint() }))
	require.Equalf(t, 1, int(testing.AllocsPerRun(100, func() { overBuffer.Fingerprint() })), "one exact allocation past the stack buffer")
	require.Zero(t, testing.AllocsPerRun(100, func() { overBuffer.FingerprintForLabels(allNames...) }))
	require.Zero(t, testing.AllocsPerRun(100, func() { overBuffer.FingerprintForLabels(mostlyAbsent[:2]...) }))
	require.Zero(t, testing.AllocsPerRun(100, func() { atBuffer.FingerprintForLabels(mostlyAbsent...) }))
}

func TestMetricToFastFingerprint(t *testing.T) {
	scenarios := []struct {
		in  LabelSet
		out Fingerprint
	}{
		{
			in:  LabelSet{},
			out: 14695981039346656037,
		},
		{
			in:  LabelSet{"name": "garland, briggs", "fear": "love is not enough"},
			out: 12952432476264840823,
		},
	}

	for i, scenario := range scenarios {
		actual := labelSetToFastFingerprint(scenario.in)

		if actual != scenario.out {
			t.Errorf("%d. expected %d, got %d", i, scenario.out, actual)
		}
	}
}

func TestSignatureForLabels(t *testing.T) {
	scenarios := []struct {
		in     Metric
		labels LabelNames
		out    uint64
	}{
		{
			in:     Metric{},
			labels: nil,
			out:    14695981039346656037,
		},
		{
			in:     Metric{},
			labels: LabelNames{"empty"},
			out:    7187873163539638612,
		},
		{
			in:     Metric{"name": "garland, briggs", "fear": "love is not enough"},
			labels: LabelNames{"empty"},
			out:    7187873163539638612,
		},
		{
			in:     Metric{"name": "garland, briggs", "fear": "love is not enough"},
			labels: LabelNames{"fear", "name"},
			out:    5799056148416392346,
		},
		{
			in:     Metric{"name": "garland, briggs", "fear": "love is not enough", "foo": "bar"},
			labels: LabelNames{"fear", "name"},
			out:    5799056148416392346,
		},
		{
			in:     Metric{"name": "garland, briggs", "fear": "love is not enough"},
			labels: LabelNames{},
			out:    14695981039346656037,
		},
		{
			in:     Metric{"name": "garland, briggs", "fear": "love is not enough"},
			labels: nil,
			out:    14695981039346656037,
		},
	}

	for i, scenario := range scenarios {
		actual := SignatureForLabels(scenario.in, scenario.labels...)

		if actual != scenario.out {
			t.Errorf("%d. expected %d, got %d", i, scenario.out, actual)
		}
	}
}

func TestSignatureWithoutLabels(t *testing.T) {
	scenarios := []struct {
		in     Metric
		labels map[LabelName]struct{}
		out    uint64
	}{
		{
			in:     Metric{},
			labels: nil,
			out:    14695981039346656037,
		},
		{
			in:     Metric{"name": "garland, briggs", "fear": "love is not enough"},
			labels: map[LabelName]struct{}{"fear": {}, "name": {}},
			out:    14695981039346656037,
		},
		{
			in:     Metric{"name": "garland, briggs", "fear": "love is not enough", "foo": "bar"},
			labels: map[LabelName]struct{}{"foo": {}},
			out:    5799056148416392346,
		},
		{
			in:     Metric{"name": "garland, briggs", "fear": "love is not enough"},
			labels: map[LabelName]struct{}{},
			out:    5799056148416392346,
		},
		{
			in:     Metric{"name": "garland, briggs", "fear": "love is not enough"},
			labels: nil,
			out:    5799056148416392346,
		},
	}

	for i, scenario := range scenarios {
		actual := SignatureWithoutLabels(scenario.in, scenario.labels)

		if actual != scenario.out {
			t.Errorf("%d. expected %d, got %d", i, scenario.out, actual)
		}
	}
}

func benchmarkLabelToSignature(b *testing.B, l map[string]string, e uint64) {
	for i := 0; i < b.N; i++ {
		a := LabelsToSignature(l)
		require.Equalf(b, a, e, "expected signature of %d for %s, got %d", e, l, a)
	}
}

func BenchmarkLabelToSignatureScalar(b *testing.B) {
	benchmarkLabelToSignature(b, nil, 14695981039346656037)
}

func BenchmarkLabelToSignatureSingle(b *testing.B) {
	benchmarkLabelToSignature(b, map[string]string{"first-label": "first-label-value"}, 5146282821936882169)
}

func BenchmarkLabelToSignatureDouble(b *testing.B) {
	benchmarkLabelToSignature(b, map[string]string{"first-label": "first-label-value", "second-label": "second-label-value"}, 3195800080984914717)
}

func BenchmarkLabelToSignatureTriple(b *testing.B) {
	benchmarkLabelToSignature(b, map[string]string{"first-label": "first-label-value", "second-label": "second-label-value", "third-label": "third-label-value"}, 13843036195897128121)
}

func benchmarkMetricToFingerprint(b *testing.B, ls LabelSet, e Fingerprint) {
	b.ReportAllocs()
	for b.Loop() {
		if a := labelSetToFingerprint(ls); a != e {
			b.Fatalf("expected signature of %d for %s, got %d", e, ls, a)
		}
	}
}

func BenchmarkMetricToFingerprintScalar(b *testing.B) {
	benchmarkMetricToFingerprint(b, nil, 14695981039346656037)
}

func BenchmarkMetricToFingerprintSingle(b *testing.B) {
	benchmarkMetricToFingerprint(b, LabelSet{"first-label": "first-label-value"}, 5146282821936882169)
}

func BenchmarkMetricToFingerprintDouble(b *testing.B) {
	benchmarkMetricToFingerprint(b, LabelSet{"first-label": "first-label-value", "second-label": "second-label-value"}, 3195800080984914717)
}

func BenchmarkMetricToFingerprintTriple(b *testing.B) {
	benchmarkMetricToFingerprint(b, LabelSet{"first-label": "first-label-value", "second-label": "second-label-value", "third-label": "third-label-value"}, 13843036195897128121)
}

// BenchmarkMetricToFingerprintTwenty uses a label count typical of alerts
// carrying external and Kubernetes labels.
func BenchmarkMetricToFingerprintTwenty(b *testing.B) {
	ls := LabelSet{}
	m := map[string]string{}
	for i := range 20 {
		ls[LabelName("label_"+strconv.Itoa(i))] = LabelValue("value_" + strconv.Itoa(i))
		m["label_"+strconv.Itoa(i)] = "value_" + strconv.Itoa(i)
	}
	benchmarkMetricToFingerprint(b, ls, Fingerprint(LabelsToSignature(m)))
}

func BenchmarkLabelSetFingerprintForLabels(b *testing.B) {
	ls := LabelSet{}
	for i := range 12 {
		ls[LabelName(fmt.Sprintf("label_%02d", i))] = LabelValue("value_" + strconv.Itoa(i))
	}
	for _, n := range []int{1, 3, 8} {
		sorted := make([]LabelName, 0, n)
		for i := range n {
			sorted = append(sorted, LabelName(fmt.Sprintf("label_%02d", i)))
		}
		unsorted := slices.Clone(sorted)
		slices.Reverse(unsorted)
		b.Run(fmt.Sprintf("%d names sorted", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				ls.FingerprintForLabels(sorted...)
			}
		})
		if n == 1 {
			continue
		}
		b.Run(fmt.Sprintf("%d names unsorted", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				ls.FingerprintForLabels(unsorted...)
			}
		})
	}
}

func benchmarkMetricToFastFingerprint(b *testing.B, ls LabelSet, e Fingerprint) {
	for i := 0; i < b.N; i++ {
		a := labelSetToFastFingerprint(ls)
		require.Equalf(b, a, e, "expected signature of %d for %s, got %d", e, ls, a)
	}
}

func BenchmarkMetricToFastFingerprintScalar(b *testing.B) {
	benchmarkMetricToFastFingerprint(b, nil, 14695981039346656037)
}

func BenchmarkMetricToFastFingerprintSingle(b *testing.B) {
	benchmarkMetricToFastFingerprint(b, LabelSet{"first-label": "first-label-value"}, 5147259542624943964)
}

func BenchmarkMetricToFastFingerprintDouble(b *testing.B) {
	benchmarkMetricToFastFingerprint(b, LabelSet{"first-label": "first-label-value", "second-label": "second-label-value"}, 18269973311206963528)
}

func BenchmarkMetricToFastFingerprintTriple(b *testing.B) {
	benchmarkMetricToFastFingerprint(b, LabelSet{"first-label": "first-label-value", "second-label": "second-label-value", "third-label": "third-label-value"}, 15738406913934009676)
}

func BenchmarkEmptyLabelSignature(b *testing.B) {
	input := []map[string]string{nil, {}}

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	alloc := ms.Alloc

	for _, labels := range input {
		LabelsToSignature(labels)
	}

	runtime.ReadMemStats(&ms)

	got := ms.Alloc
	require.Equalf(b, alloc, got, "expected LabelsToSignature with empty labels not to perform allocations")
}

func benchmarkMetricToFastFingerprintConc(b *testing.B, ls LabelSet, e Fingerprint, concLevel int) {
	var start, end sync.WaitGroup
	start.Add(1)
	end.Add(concLevel)
	errc := make(chan error, 1)

	for range concLevel {
		go func() {
			start.Wait()
			for j := b.N / concLevel; j >= 0; j-- {
				if a := labelSetToFastFingerprint(ls); a != e {
					select {
					case errc <- fmt.Errorf("expected signature of %d for %s, got %d", e, ls, a):
					default:
					}
				}
			}
			end.Done()
		}()
	}
	b.ResetTimer()
	start.Done()
	end.Wait()

	select {
	case err := <-errc:
		b.Fatal(err)
	default:
	}
}

func BenchmarkMetricToFastFingerprintTripleConc1(b *testing.B) {
	benchmarkMetricToFastFingerprintConc(b, LabelSet{"first-label": "first-label-value", "second-label": "second-label-value", "third-label": "third-label-value"}, 15738406913934009676, 1)
}

func BenchmarkMetricToFastFingerprintTripleConc2(b *testing.B) {
	benchmarkMetricToFastFingerprintConc(b, LabelSet{"first-label": "first-label-value", "second-label": "second-label-value", "third-label": "third-label-value"}, 15738406913934009676, 2)
}

func BenchmarkMetricToFastFingerprintTripleConc4(b *testing.B) {
	benchmarkMetricToFastFingerprintConc(b, LabelSet{"first-label": "first-label-value", "second-label": "second-label-value", "third-label": "third-label-value"}, 15738406913934009676, 4)
}

func BenchmarkMetricToFastFingerprintTripleConc8(b *testing.B) {
	benchmarkMetricToFastFingerprintConc(b, LabelSet{"first-label": "first-label-value", "second-label": "second-label-value", "third-label": "third-label-value"}, 15738406913934009676, 8)
}

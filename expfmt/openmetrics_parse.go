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
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/prometheus/common/model"
)

const (
	OpenMetricTypeCounter        = "counter"
	OpenMetricTypeGauge          = "gauge"
	OpenMetricTypeHistogram      = "histogram"
	OpenMetricTypeGaugeHistogram = "gaugehistogram"
	OpenMetricTypeSummary        = "summary"
	OpenMetricTypeInfo           = "info"
	OpenMetricTypeStateset       = "stateset"
	OpenMetricTypeUnknown        = "unknown"
)

var ValidOpenMetricType = map[string]struct{}{
	OpenMetricTypeCounter:        {},
	OpenMetricTypeGauge:          {},
	OpenMetricTypeHistogram:      {},
	OpenMetricTypeGaugeHistogram: {},
	OpenMetricTypeSummary:        {},
	OpenMetricTypeInfo:           {},
	OpenMetricTypeStateset:       {},
	OpenMetricTypeUnknown:        {},
}

type OpenMetricFamily struct {
	Name       *string
	Help       *string
	Unit       *string
	MetricType *string
	Samples    []*Sample
}

type Sample struct {
	Name      string
	Value     Value
	Labels    map[string]string
	Timestamp *Timestamp
	Exemplar  *Exemplar
}

type Exemplar struct {
	Value     Value
	Labels    map[string]string
	Timestamp *Timestamp
}

type Timestamp struct {
	Sec  int64
	NSec int64
}

type Value float64

type OpenMetricsParser struct {
	metricFamiliesByName     map[string]*OpenMetricFamily
	buf                      *bufio.Reader // Where the parsed input is read through.
	err                      error         // Most recent error.
	lineCount                int           // Tracks the line count for error messages.
	currentByte              byte          // The most recent byte read.
	currentToken             bytes.Buffer  // Re-used each time a token has to be gathered from multiple bytes.
	currentMF                *OpenMetricFamily
	currentSample            *Sample
	currentLabelName         string
	currentExemplarLabelName string
	isReadingExemplar        bool
	exemplarLength           int  // The exemplar length.
	seenEofLine              bool // OpenMetrics must end with EOF.
}

// TextToOpenMetricFamilies reads 'in' as the simple and text OpenMetrics
// format and creates MetricSet. It returns the MetricFamily
// in a map where the metric names are the keys, along with any
// error encountered.
//
// If the input contains duplicate metrics (i.e. lines with the same metric name
// and exactly the same label set), the resulting MetricFamily will contain
// duplicate Metric. Similar is true for duplicate label names.
// Checks for duplicates have to be performed separately, if required.
// Also note that neither the metrics within each MetricFamily are sorted nor
// the label pairs within each Metric. This is a laxer parser than the main Go parser,
// so successful parsing does not imply that the text meets the OpenMetrics specification.
//
// This method must not be called concurrently. If you want to parse different
// input concurrently, instantiate a separate Parser for each goroutine.
func (p *OpenMetricsParser) TextToOpenMetricFamilies(in io.Reader) (map[string]*OpenMetricFamily, error) {
	p.reset(in)
	for nextState := p.startOfLine; nextState != nil; nextState = nextState() {
		// Magic happens here...
	}
	// If p.err is io.EOF now, we have run into a premature end of the input
	// stream. Turn this error into something nicer and more
	// meaningful. (io.EOF is often used as a signal for the legitimate end
	// of an input stream.)
	if p.err == io.EOF {
		p.parseError("unexpected end of input stream")
	}
	if p.err == nil && !p.seenEofLine {
		p.parseError("expected '# EOF' at end")
	}
	// Get rid of empty metric families.
	for k, mf := range p.metricFamiliesByName {
		if mf.GetSample() == 0 {
			delete(p.metricFamiliesByName, k)
		}
	}
	return p.metricFamiliesByName, p.err
}

func (p *OpenMetricsParser) reset(in io.Reader) {
	if p.buf == nil {
		p.buf = bufio.NewReader(in)
	} else {
		p.buf.Reset(in)
	}
	p.metricFamiliesByName = make(map[string]*OpenMetricFamily)
	p.err = nil
	p.lineCount = 0
	p.isReadingExemplar = false
	p.exemplarLength = 0
	p.seenEofLine = false
}

// startOfLine represents the state where the next byte read from p.buf is the
// start of a line (or whitespace leading up to it).
func (p *OpenMetricsParser) startOfLine() stateFn {
	p.lineCount++
	if p.skipBlankTab(); p.err != nil {
		// End of input reached. This is the only case where
		// that is not an error but a signal that we are done.
		p.err = nil
		return nil
	}
	if p.seenEofLine {
		p.parseError("a line after '# EOF'")
		return nil
	}
	if p.currentByte == '#' {
		return p.startComment
	}
	if p.currentByte == '\n' {
		return p.startOfLine
	}
	return p.readingMetricName
}

// startComment represents the state where the next byte read from p.buf is the
// start of a comment (or whitespace leading up to it).
func (p *OpenMetricsParser) startComment() stateFn {
	if p.skipBlankTab(); p.err != nil {
		return nil // Unexpected end of input.
	}
	if p.currentByte == '\n' {
		return p.startOfLine
	}
	if p.readTokenUntilWhitespace(); p.err != nil {
		return nil // Unexpected end of input.
	}
	keyword := p.currentToken.String()
	if keyword == "EOF" {
		if p.skipBlankTabIfCurrentBlankTab(); p.err != nil {
			return nil
		}
		if p.currentByte != '\n' {
			p.parseError("invalid eof line")
			return nil
		}
		p.seenEofLine = true
		return p.startOfLine
	}
	// If we have hit the end of line already, there is nothing left
	// to do. This is not considered a syntax error.
	if p.currentByte == '\n' {
		return p.startOfLine
	}
	if keyword != "HELP" && keyword != "TYPE" && keyword != "UNIT" {
		// Generic comment, ignore by fast forwarding to end of line.
		for p.currentByte != '\n' {
			if p.currentByte, p.err = p.buf.ReadByte(); p.err != nil {
				return nil // Unexpected end of input.
			}
		}
		return p.startOfLine
	}
	// There is something. Next has to be a metric name.
	if p.skipBlankTab(); p.err != nil {
		return nil // Unexpected end of input.
	}
	if p.readTokenAsMetricName(); p.err != nil {
		return nil // Unexpected end of input.
	}
	if p.currentByte == '\n' {
		// At the end of the line already.
		// Again, this is not considered a syntax error.
		return p.startOfLine
	}
	if !isBlankOrTab(p.currentByte) {
		p.parseError("invalid metric name in comment")
		return nil
	}
	p.setOrCreateCurrentMF()
	if p.skipBlankTab(); p.err != nil {
		return nil // Unexpected end of input.
	}
	if p.currentByte == '\n' {
		// At the end of the line already.
		// Again, this is not considered a syntax error.
		return p.startOfLine
	}
	switch keyword {
	case "HELP":
		return p.readingHelp
	case "TYPE":
		return p.readingType
	case "UNIT":
		return p.readingUnit
	}
	panic(fmt.Sprintf("code error: unexpected keyword %q", keyword))
}

// readingMetricName represents the state where the last byte read (now in
// p.currentByte) is the first byte of a metric name.
func (p *OpenMetricsParser) readingMetricName() stateFn {
	if p.readTokenAsMetricName(); p.err != nil {
		return nil
	}
	if p.currentToken.Len() == 0 {
		p.parseError("invalid metric name")
		return nil
	}
	p.setOrCreateCurrentMF()
	// Now is the time to fix the type if it hasn't happened yet.
	if p.currentMF.MetricType == nil {
		p.currentMF.MetricType = String(OpenMetricTypeUnknown)
	}
	if p.currentMF.GetType() == OpenMetricTypeInfo && p.currentMF.GetUnit() != "" {
		p.parseError(fmt.Sprintf("expected Info metric have an empty unit, found %q", p.currentMF.GetUnit()))
		return nil
	}
	if p.currentMF.GetType() == OpenMetricTypeStateset && p.currentMF.GetUnit() != "" {
		p.parseError(fmt.Sprintf("expected StateSet have an metric empty unit, found %q", p.currentMF.GetUnit()))
		return nil
	}
	p.currentSample = &Sample{}
	p.currentSample.Name = p.currentToken.String()
	p.currentMF.Samples = append(p.currentMF.Samples, p.currentSample)
	if p.skipBlankTabIfCurrentBlankTab(); p.err != nil {
		return nil // Unexpected end of input.
	}
	return p.readingLabels
}

// readingLabels represents the state where the last byte read (now in
// p.currentByte) is either the first byte of the label set (i.e. a '{'), or the
// first byte of the value (otherwise).
func (p *OpenMetricsParser) readingLabels() stateFn {
	if p.currentByte != '{' {
		return p.readingValue
	}
	return p.startLabelName
}

// startLabelName represents the state where the next byte read from p.buf is
// the start of a label name (or whitespace leading up to it).
func (p *OpenMetricsParser) startLabelName() stateFn {
	if p.skipBlankTab(); p.err != nil {
		return nil
	}
	if p.currentByte == '}' {
		if p.skipBlankTab(); p.err != nil {
			return nil // Unexpected end of input.
		}
		return p.readingValue
	}
	if p.readTokenAsLabelName(); p.err != nil {
		return nil
	}
	if p.currentToken.Len() == 0 {
		p.parseError(fmt.Sprintf("invalid label name for metric %q", p.currentMF.GetName()))
		return nil
	}
	if p.skipBlankTabIfCurrentBlankTab(); p.err != nil {
		return nil
	}
	if p.currentByte != '=' {
		p.parseError(fmt.Sprintf("expected '=' after label name, found %q", p.currentByte))
		return nil
	}
	if p.isReadingExemplar {
		if p.currentSample.Exemplar.Labels == nil {
			p.currentSample.Exemplar.Labels = make(map[string]string)
		}
		p.currentExemplarLabelName = p.currentToken.String()
		p.exemplarLength += utf8.RuneCountInString(p.currentExemplarLabelName)
		if p.exemplarLength > 128 {
			p.parseError("out of exemplar max length 128")
			return nil
		}
	} else {
		if p.currentToken.String() == model.MetricNameLabel {
			p.parseError(fmt.Sprintf("label name %q is reserved", model.MetricNameLabel))
			return nil
		}
		if p.currentSample.Labels == nil {
			p.currentSample.Labels = make(map[string]string)
		}
		p.currentLabelName = p.currentToken.String()
		if _, duplicate := p.currentSample.Labels[p.currentLabelName]; duplicate {
			p.parseError(fmt.Sprintf("duplicate label names for metric %q", p.currentMF.GetName()))
			return nil
		}
	}
	return p.startLabelValue
}

// startLabelValue represents the state where the next byte read from p.buf is
// the start of a (quoted) label value (or whitespace leading up to it).
func (p *OpenMetricsParser) startLabelValue() stateFn {
	if p.skipBlankTab(); p.err != nil {
		return nil
	}
	if p.currentByte != '"' {
		p.parseError(fmt.Sprintf("expected '\"' at start of label value, found %q", p.currentByte))
		return nil
	}
	if p.readTokenAsLabelValue(); p.err != nil {
		return nil
	}
	if !model.LabelValue(p.currentToken.String()).IsValid() {
		p.parseError(fmt.Sprintf("invalid label value %q", p.currentToken.String()))
		return nil
	}
	if p.isReadingExemplar {
		p.exemplarLength += utf8.RuneCountInString(p.currentToken.String())
		if p.exemplarLength > 128 {
			p.parseError("out of exemplar max length 128")
			return nil
		}
		p.currentSample.Exemplar.Labels[p.currentExemplarLabelName] = p.currentToken.String()
	} else {
		p.currentSample.Labels[p.currentLabelName] = p.currentToken.String()
	}
	if p.skipBlankTab(); p.err != nil {
		return nil // Unexpected end of input.
	}
	switch p.currentByte {
	case '}':
		if p.skipBlankTab(); p.err != nil {
			return nil
		}
		if !p.isReadingExemplar && p.currentMF.GetType() == OpenMetricTypeStateset {
			if _, ok := p.currentSample.Labels[p.currentMF.GetName()]; !ok {
				p.parseError(fmt.Sprintf("expected label %q for StateSet metric type", p.currentMF.GetName()))
				return nil
			}
		}
		return p.readingValue
	case ',':
		return p.startLabelName
	default:
		p.parseError(fmt.Sprintf("unexpected end of label value %q", p.currentToken.String()))
		return nil
	}
}

// readingValue represents the state where the last byte read (now in
// p.currentByte) is the first byte of the sample value (i.e. a float).
func (p *OpenMetricsParser) readingValue() stateFn {
	if p.readTokenUntilWhiteSpace(); p.err != nil {
		return nil
	}
	value, err := parseFloat(p.currentToken.String())
	if err != nil {
		p.parseError(fmt.Sprintf("expected float as value, got %q", p.currentToken.String()))
		return nil
	}
	if p.isReadingExemplar {
		p.currentSample.Exemplar.Value = Value(value)
		if p.currentByte == '\n' {
			p.isReadingExemplar = false
			return p.startOfLine
		}
		if p.skipBlankTab(); p.err != nil {
			return nil
		}
		return p.startTimestamp
	}
	switch *p.currentMF.MetricType {
	case OpenMetricTypeCounter:

	case OpenMetricTypeGauge:

	case OpenMetricTypeSummary:
		mn := p.currentSample.Name
		if isOpenMetricCount(mn) && (value < 0 || math.IsNaN(value)) {
			p.parseError(fmt.Sprintf("expected summary count value not be NaN or negtive, got %f", value))
			return nil
		}
		if isOpenMetricSum(mn) && (value < 0 || math.IsNaN(value)) {
			p.parseError(fmt.Sprintf("expected summary sum value not be NaN or negtive, got %f", value))
			return nil
		}
		if quantile, ok := p.currentSample.Labels[model.QuantileLabel]; ok {
			if value, err := parseFloat(quantile); err != nil {
				p.parseError(fmt.Sprintf("expected float as value for 'quantile' label, got %q", quantile))
				return nil
			} else {
				if value < 0 || value > 1 {
					p.parseError(fmt.Sprintf("expected summary quantile between 0 and 1 inclusive, got %f", value))
					return nil
				}
			}
		}
	case OpenMetricTypeHistogram:
		mn := p.currentSample.Name
		if isOpenMetricSum(mn) && (value < 0 || math.IsNaN(value)) {
			p.parseError(fmt.Sprintf("expected histogram sum value not be NaN or negtive, got %f", value))
			return nil
		}
		if isOpenMetricBucket(mn) {
			if value < 0 || math.IsNaN(value) {
				p.parseError(fmt.Sprintf("expected histogram bucket value not be NaN or negtive, got %f", value))
				return nil
			}
			if le, ok := p.currentSample.Labels[model.BucketLabel]; !ok {
				p.parseError("expected histogram bucket 'le' label, got empty")
				return nil
			} else {
				if _, err := parseFloat(le); err != nil {
					p.parseError(fmt.Sprintf("expected float as value for 'le' label, got %q", le))
					return nil
				}
			}
		}
	case OpenMetricTypeGaugeHistogram:
		mn := p.currentSample.Name
		if isOpenMetricGSum(mn) && math.IsNaN(value) {
			p.parseError(fmt.Sprintf("expected gaugehistogram sum value not be NaN, got %f", value))
			return nil
		}
		if isOpenMetricBucket(mn) {
			if value < 0 || math.IsNaN(float64(p.currentSample.Value)) {
				p.parseError(fmt.Sprintf("expected histogram bucket value not be NaN or negtive, got %f", value))
				return nil
			}
			if le, ok := p.currentSample.Labels[model.BucketLabel]; !ok {
				p.parseError("expected histogram bucket 'le' label, got empty")
				return nil
			} else {
				if _, err := parseFloat(le); err != nil {
					p.parseError(fmt.Sprintf("expected float as value for 'le' label, got %q", le))
					return nil
				}
			}
		}
	case OpenMetricTypeStateset:
		if value != 0 && value != 1 {
			p.parseError(fmt.Sprintf("stateSet value must be 0 or 1, got %f", value))
			return nil
		}
	case OpenMetricTypeInfo:
		if value != 1 {
			p.parseError(fmt.Sprintf("info value must be 1, got %f", value))
			return nil
		}
	case OpenMetricTypeUnknown:
	default:
		p.err = fmt.Errorf("unexpected type for metric name %q", p.currentMF.GetName())
		return nil
	}
	p.currentSample.Value = Value(value)
	if p.currentByte == '\n' {
		return p.startOfLine
	}
	if p.skipBlankTab(); p.err != nil {
		return nil
	}
	switch p.currentByte {
	case '#':
		return p.startExemplar
	default:
		return p.startTimestamp
	}
}

// startExemplar represents the state where the next byte read from p.buf is
// the start of the exemplar (or whitespace leading up to it).
func (p *OpenMetricsParser) startExemplar() stateFn {
	if p.skipBlankTab(); p.err != nil {
		return nil
	}
	if p.currentSample.Exemplar != nil {
		p.parseError(fmt.Sprintf("second exemplar for metric name: %q", p.currentSample.Name))
		return nil
	}
	p.isReadingExemplar = true
	p.exemplarLength = 0
	exemplar := &Exemplar{}
	p.currentSample.Exemplar = exemplar
	return p.startLabelName
}

// startTimestamp represents the state where the next byte read from p.buf is
// the start of the timestamp (or whitespace leading up to it).
func (p *OpenMetricsParser) startTimestamp() stateFn {
	if p.readTokenUntilWhitespace(); p.err != nil {
		return nil // Unexpected end of input.
	}
	timestamp, err := parseTimestamp(p.currentToken.String())
	if err != nil {
		p.parseError(fmt.Sprintf("invalid timestamp format %q", p.currentToken.String()))
		return nil
	}
	if p.isReadingExemplar {
		p.currentSample.Exemplar.Timestamp = timestamp
	} else {
		p.currentSample.Timestamp = timestamp
	}
	if p.skipBlankTabIfCurrentBlankTab(); p.err != nil {
		return nil
	}
	switch p.currentByte {
	case '#':
		return p.startExemplar
	case '\n':
		if p.isReadingExemplar {
			p.isReadingExemplar = false
		}
		return p.startOfLine
	default:
		p.parseError(fmt.Sprintf("unexpected byte '%c' after timestamp", p.currentByte))
		return nil
	}
}

// readingHelp represents the state where the last byte read (now in
// p.currentByte) is the first byte of the docstring after 'HELP'.
func (p *OpenMetricsParser) readingHelp() stateFn {
	if p.currentMF.Help != nil {
		p.parseError(fmt.Sprintf("second HELP line for metric name %q", p.currentMF.GetName()))
		return nil
	}
	// Rest of line is the docstring.
	if p.readTokenUntilNewline(true); p.err != nil {
		return nil // Unexpected end of input.
	}
	p.currentMF.Help = String(p.currentToken.String())
	return p.startOfLine
}

// readingType represents the state where the last byte read (now in
// p.currentByte) is the first byte of the type hint after 'TYPE'.
func (p *OpenMetricsParser) readingType() stateFn {
	if p.currentMF.MetricType != nil {
		p.parseError(fmt.Sprintf("second TYPE line for metric name %q, or TYPE reported after samples", p.currentMF.GetName()))
		return nil
	}
	if p.readTokenUntilNewline(false); p.err != nil {
		return nil
	}
	if _, ok := ValidOpenMetricType[p.currentToken.String()]; !ok {
		p.parseError(fmt.Sprintf("unknown metric type %q", p.currentToken.String()))
		return nil
	}
	p.currentMF.MetricType = String(p.currentToken.String())
	return p.startOfLine
}

// readingUnit represents the state where the last byte read (now in
// p.currentByte) is the first byte of the unit hint after 'UNIT'.
func (p *OpenMetricsParser) readingUnit() stateFn {
	if p.currentMF.Unit != nil {
		p.parseError(fmt.Sprintf("second UNIT line for metric name %q", p.currentMF.GetName()))
		return nil
	}
	if p.readTokenUntilNewline(false); p.err != nil {
		return nil
	}
	if !strings.HasSuffix(p.currentMF.GetName(), p.currentToken.String()) {
		p.parseError(fmt.Sprintf("expected unit as metric name suffix, found %q", p.currentMF.GetName()))
		return nil
	}
	p.currentMF.Unit = String(p.currentToken.String())
	return p.startOfLine
}

// parseError sets p.err to a ParseError at the current line with the given
// message.
func (p *OpenMetricsParser) parseError(msg string) {
	p.err = ParseError{
		Line: p.lineCount,
		Msg:  msg,
	}
}

// skipBlankTab reads (and discards) bytes from p.buf until it encounters a byte
// that is neither ' ' nor '\t'. That byte is left in p.currentByte.
func (p *OpenMetricsParser) skipBlankTab() {
	for {
		if p.currentByte, p.err = p.buf.ReadByte(); p.err != nil || !isBlankOrTab(p.currentByte) {
			return
		}
	}
}

// readTokenUntilWhitespace copies bytes from p.buf into p.currentToken.  The
// first byte considered is the byte already read (now in p.currentByte).  The
// first whitespace byte encountered is still copied into p.currentByte, but not
// into p.currentToken.
func (p *OpenMetricsParser) readTokenUntilWhiteSpace() {
	p.currentToken.Reset()
	for p.err == nil && !isBlankOrTab(p.currentByte) && p.currentByte != '\n' {
		p.currentToken.WriteByte(p.currentByte)
		p.currentByte, p.err = p.buf.ReadByte()
	}
}

// readTokenUntilNewline copies bytes from p.buf into p.currentToken.  The first
// byte considered is the byte already read (now in p.currentByte).  The first
// newline byte encountered is still copied into p.currentByte, but not into
// p.currentToken. If recognizeEscapeSequence is true, two escape sequences are
// recognized: '\\' translates into '\', and '\n' into a line-feed character.
// All other escape sequences are invalid and cause an error.
func (p *OpenMetricsParser) readTokenUntilNewline(recognizeEscapeSequence bool) {
	p.currentToken.Reset()
	escaped := false
	for p.err == nil {
		if recognizeEscapeSequence && escaped {
			switch p.currentByte {
			case '\\':
				p.currentToken.WriteByte(p.currentByte)
			case 'n':
				p.currentToken.WriteByte('\n')
			default:
				p.parseError(fmt.Sprintf("invalid escape sequence '\\%c'", p.currentByte))
				return
			}
			escaped = false
		} else {
			switch p.currentByte {
			case '\n':
				return
			case '\\':
				escaped = true
			default:
				p.currentToken.WriteByte(p.currentByte)
			}
		}
		p.currentByte, p.err = p.buf.ReadByte()
	}
}

// skipBlankTabIfCurrentBlankTab works exactly as skipBlankTab but doesn't do
// anything if p.currentByte is neither ' ' nor '\t'.
func (p *OpenMetricsParser) skipBlankTabIfCurrentBlankTab() {
	if isBlankOrTab(p.currentByte) {
		p.skipBlankTab()
	}
}

// readTokenUntilWhitespace copies bytes from p.buf into p.currentToken.  The
// first byte considered is the byte already read (now in p.currentByte).  The
// first whitespace byte encountered is still copied into p.currentByte, but not
// into p.currentToken.
func (p *OpenMetricsParser) readTokenUntilWhitespace() {
	p.currentToken.Reset()
	for p.err == nil && !isBlankOrTab(p.currentByte) && p.currentByte != '\n' {
		p.currentToken.WriteByte(p.currentByte)
		p.currentByte, p.err = p.buf.ReadByte()
	}
}

// readTokenAsMetricName copies a metric name from p.buf into p.currentToken.
// The first byte considered is the byte already read (now in p.currentByte).
// The first byte not part of a metric name is still copied into p.currentByte,
// but not into p.currentToken.
func (p *OpenMetricsParser) readTokenAsMetricName() {
	p.currentToken.Reset()
	if !isValidMetricNameStart(p.currentByte) {
		return
	}
	for {
		p.currentToken.WriteByte(p.currentByte)
		p.currentByte, p.err = p.buf.ReadByte()
		if p.err != nil || !isValidMetricNameContinuation(p.currentByte) {
			return
		}
	}
}

// readTokenAsLabelName copies a label name from p.buf into p.currentToken.
// The first byte considered is the byte already read (now in p.currentByte).
// The first byte not part of a label name is still copied into p.currentByte,
// but not into p.currentToken.
func (p *OpenMetricsParser) readTokenAsLabelName() {
	p.currentToken.Reset()
	if !isValidLabelNameStart(p.currentByte) {
		return
	}
	for {
		p.currentToken.WriteByte(p.currentByte)
		p.currentByte, p.err = p.buf.ReadByte()
		if p.err != nil || !isValidLabelNameContinuation(p.currentByte) {
			return
		}
	}
}

// readTokenAsLabelValue copies a label value from p.buf into p.currentToken.
// In contrast to the other 'readTokenAs...' functions, which start with the
// last read byte in p.currentByte, this method ignores p.currentByte and starts
// with reading a new byte from p.buf. The first byte not part of a label value
// is still copied into p.currentByte, but not into p.currentToken.
func (p *OpenMetricsParser) readTokenAsLabelValue() {
	p.currentToken.Reset()
	escaped := false
	for {
		if p.currentByte, p.err = p.buf.ReadByte(); p.err != nil {
			return
		}
		if escaped {
			switch p.currentByte {
			case '"', '\\':
				p.currentToken.WriteByte(p.currentByte)
			case 'n':
				p.currentToken.WriteByte('\n')
			default:
				p.parseError(fmt.Sprintf("invalid escape sequence '\\%c'", p.currentByte))
				return
			}
			escaped = false
			continue
		}
		switch p.currentByte {
		case '"':
			return
		case '\n':
			p.parseError(fmt.Sprintf("label value %q contains unescaped new-line", p.currentToken.String()))
			return
		case '\\':
			escaped = true
		default:
			p.currentToken.WriteByte(p.currentByte)
		}
	}
}

func (p *OpenMetricsParser) setOrCreateCurrentMF() {
	name := p.currentToken.String()
	counterName := OpenMetricCounterName(name)
	if p.currentMF = p.metricFamiliesByName[counterName]; p.currentMF != nil {
		if p.currentMF.GetType() == OpenMetricTypeCounter {
			return
		}
	}
	summaryName := OpenMetricSummaryName(name)
	if p.currentMF = p.metricFamiliesByName[summaryName]; p.currentMF != nil {
		if p.currentMF.GetType() == OpenMetricTypeSummary {
			return
		}
	}
	histogramName := OpenMetricHistogramName(name)
	if p.currentMF = p.metricFamiliesByName[histogramName]; p.currentMF != nil {
		if p.currentMF.GetType() == OpenMetricTypeHistogram {
			return
		}
	}
	gaugeHistogramName := OpenMetricGaugeHistogramName(name)
	if p.currentMF = p.metricFamiliesByName[gaugeHistogramName]; p.currentMF != nil {
		if p.currentMF.GetType() == OpenMetricTypeGaugeHistogram {
			return
		}
	}
	infoName := OpenMetricInfoName(name)
	if p.currentMF = p.metricFamiliesByName[infoName]; p.currentMF != nil {
		if p.currentMF.GetType() == OpenMetricTypeInfo {
			return
		}
	}
	if p.currentMF = p.metricFamiliesByName[name]; p.currentMF != nil {
		return
	}
	p.currentMF = &OpenMetricFamily{Name: &name}
	p.metricFamiliesByName[name] = p.currentMF
}

func OpenMetricCounterName(name string) string {
	switch {
	case isOpenMetricCreated(name):
		return name[:len(name)-8]
	case isOpenMetricTotal(name):
		return name[:len(name)-6]
	default:
		return name
	}
}

func OpenMetricHistogramName(name string) string {
	switch {
	case isOpenMetricCount(name):
		return name[:len(name)-6]
	case isOpenMetricSum(name):
		return name[:len(name)-4]
	case isOpenMetricCreated(name):
		return name[:len(name)-8]
	case isOpenMetricBucket(name):
		return name[:len(name)-7]

	default:
		return name
	}
}

func OpenMetricSummaryName(name string) string {
	switch {
	case isOpenMetricCount(name):
		return name[:len(name)-6]
	case isOpenMetricSum(name):
		return name[:len(name)-4]
	case isOpenMetricCreated(name):
		return name[:len(name)-8]
	default:
		return name
	}
}

func OpenMetricGaugeHistogramName(name string) string {
	switch {
	case isOpenMetricGCount(name):
		return name[:len(name)-7]
	case isOpenMetricGSum(name):
		return name[:len(name)-5]
	case isOpenMetricBucket(name):
		return name[:len(name)-7]
	case isOpenMetricCreated(name):
		return name[:len(name)-8]
	default:
		return name
	}
}

func OpenMetricInfoName(name string) string {
	switch {
	case isOpenMetricInfo(name):
		return name[:len(name)-5]
	default:
		return name
	}
}

func isOpenMetricTotal(name string) bool {
	return len(name) > 6 && name[len(name)-6:] == "_total"
}

func isOpenMetricCreated(name string) bool {
	return len(name) > 8 && name[len(name)-8:] == "_created"
}

func isOpenMetricCount(name string) bool {
	return len(name) > 6 && name[len(name)-6:] == "_count"
}
func isOpenMetricSum(name string) bool {
	return len(name) > 4 && name[len(name)-4:] == "_sum"
}

func isOpenMetricBucket(name string) bool {
	return len(name) > 7 && name[len(name)-7:] == "_bucket"
}

func isOpenMetricGSum(name string) bool {
	return len(name) > 5 && name[len(name)-5:] == "_gsum"
}

func isOpenMetricGCount(name string) bool {
	return len(name) > 7 && name[len(name)-7:] == "_gcount"
}

func isOpenMetricInfo(name string) bool {
	return len(name) > 5 && name[len(name)-5:] == "_info"
}

func String(s string) *string {
	return &s
}

func parseTimestamp(timestamp string) (*Timestamp, error) {
	parts := strings.SplitN(timestamp, ".", 2)
	var sec, nsec int64
	var err error
	if sec, err = strconv.ParseInt(parts[0], 10, 64); err != nil {
		return nil, err
	}
	// aaaa.bbbb. Nanosecond resolution supported.
	if len(parts) == 2 {
		for 9-len(parts[1]) > 0 {
			parts[1] = parts[1] + "0"
		}
		parts[1] = parts[1][:9]
		if nsec, err = strconv.ParseInt(parts[1], 10, 32); err != nil {
			return nil, err
		}
		if nsec >= 1e9 {
			return nil, fmt.Errorf("invalid value for nanoseconds in Timestamp: %d", nsec)
		}
	}
	return &Timestamp{Sec: sec, NSec: nsec}, nil
}

// Implement json Marshaler interface to support marshal +Inf,-Inf,NaN.
func (v Value) MarshalJSON() ([]byte, error) {
	vs := strconv.AppendFloat(nil, float64(v), 'f', 1, 64)
	return []byte(`"` + string(vs) + `"`), nil
}

func (o *OpenMetricFamily) GetName() string {
	if o.Name != nil {
		return *o.Name
	}
	return ""
}

func (o *OpenMetricFamily) GetSample() int {
	return len(o.Samples)
}

func (o *OpenMetricFamily) GetType() string {
	if o.MetricType != nil {
		return *o.MetricType
	}
	return OpenMetricTypeUnknown
}

func (o *OpenMetricFamily) GetUnit() string {
	if o.Unit != nil {
		return *o.Unit
	}
	return ""
}

func (o *OpenMetricFamily) GetHelp() string {
	if o.Help != nil {
		return *o.Help
	}
	return ""
}

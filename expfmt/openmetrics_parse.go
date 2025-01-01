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

package expfmt

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	dto "github.com/prometheus/client_model/go"

	"google.golang.org/protobuf/proto"
)

var UnsupportMetricType = map[string]struct{}{
	"INFO":     {},
	"STATESET": {},
}

// OpenMetricsParser is used to parse the simple and flat openmetrics-based exchange format. Its
// zero value is ready to use.
type OpenMetricsParser struct {
	metricFamiliesByName map[string]*dto.MetricFamily
	buf                  *bufio.Reader // Where the parsed input is read through.
	err                  error         // Most recent error.
	lineCount            int           // Tracks the line count for error messages.
	currentByte          byte          // The most recent byte read.
	currentToken         bytes.Buffer  // Re-used each time a token has to be gathered from multiple bytes.
	currentMF            *dto.MetricFamily
	currentMetric        *dto.Metric

	// This tell us if the currently processed line ends on '_created',
	// representing the created timestamp of the metric
	currentIsMetricCreated bool

	// This tell us have read 'EOF' line, representing the end of the metrics
	currentIsEOF bool
}

// OpenMetricsToMetricFamilies reads 'in' as the simple and flat openmetrics-based
// exchange format and creates MetricFamily proto messages. It returns the MetricFamily
// proto messages in a map where the metric names are the keys, along with any
// error encountered.
//
// If the input contains duplicate metrics (i.e. lines with the same metric name
// and exactly the same label set), the resulting MetricFamily will contain
// duplicate Metric proto messages. Similar is true for duplicate label
// names. Checks for duplicates have to be performed separately, if required.
// Also note that neither the metrics within each MetricFamily are sorted nor
// the label pairs within each Metric. Sorting is not required for the most
// frequent use of this method, which is sample ingestion in the Prometheus
// server. However, for presentation purposes, you might want to sort the
// metrics, and in some cases, you must sort the labels, e.g. for consumption by
// the metric family injection hook of the Prometheus registry.
//
// Summaries and histograms are rather special beasts. You would probably not
// use them in the simple openmetrics format anyway. This method can deal with
// summaries and histograms if they are presented in exactly the way the
// openmetrics.Create function creates them.
//
// This method must not be called concurrently. If you want to parse different
// input concurrently, instantiate a separate Parser for each goroutine.
func (p *OpenMetricsParser) OpenMetricsToMetricFamilies(in io.Reader) (map[string]*dto.MetricFamily, error) {
	p.reset(in)
	p.currentIsEOF = false
	for nextState := p.startOfLine; nextState != nil; nextState = nextState() {
		// Magic happens here...
	}
	// If p.err is io.EOF now, we have run into a premature end of the input
	// stream. Turn this error into something nicer and more
	// meaningful. (io.EOF is often used as a signal for the legitimate end
	// of an input stream.)
	if p.err != nil && errors.Is(p.err, io.EOF) {
		p.parseError("unexpected end of input stream")
	}
	return p.metricFamiliesByName, p.err
}

func (p *OpenMetricsParser) reset(in io.Reader) {
	p.metricFamiliesByName = map[string]*dto.MetricFamily{}
	if p.buf == nil {
		p.buf = bufio.NewReader(in)
	} else {
		p.buf.Reset(in)
	}
	p.err = nil
	p.lineCount = 0
}

// startOfLine represents the state where the next byte read from p.buf is the
// start of a line (or whitespace leading up to it).
func (p *OpenMetricsParser) startOfLine() stateFn {
	p.lineCount++
	p.currentByte, p.err = p.buf.ReadByte()
	if p.err != nil {
		// This is the only place that we expect to see io.EOF,
		// which is not an error but the signal that we are done.
		// Any other error that happens to align with the start of
		// a line is still an error.
		if errors.Is(p.err, io.EOF) {
			if p.currentIsEOF {
				p.err = nil
			} else {
				p.parseError("expected EOF keyword at the end")
			}
		}
		return nil
	}
	if p.currentIsEOF {
		p.parseError(fmt.Sprintf("unexpected line after EOF, got %q", p.currentByte))
		return nil
	}
	switch p.currentByte {
	case '#':
		return p.startComment
	case '\n':
		return p.startOfLine // Empty line, start the next one.
	}
	if !isValidMetricNameStart(p.currentByte) {
		p.parseError(fmt.Sprintf("%q is not a valid start token", p.currentByte))
		return nil
	}
	return nil
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
	// If we have hit the end of line already, there is nothing left
	// to do. This is not considered a syntax error.
	if p.currentByte == '\n' && p.currentToken.String() != "EOF" {
		return p.startOfLine
	}
	keyword := p.currentToken.String()
	if keyword != "HELP" && keyword != "TYPE" && keyword != "UNIT" && keyword != "EOF" {
		// Generic comment, ignore by fast forwarding to end of line.
		for p.currentByte != '\n' {
			if p.currentByte, p.err = p.buf.ReadByte(); p.err != nil {
				return nil // Unexpected end of input.
			}
		}
		return p.startOfLine
	}
	if keyword == "EOF" {
		p.currentIsEOF = true
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
	p.currentMF.Help = proto.String(p.currentToken.String())
	return p.startOfLine
}

// readingType represents the state where the last byte read (now in
// p.currentByte) is the first byte of the type hint after 'HELP'.
func (p *OpenMetricsParser) readingType() stateFn {
	if p.currentMF.Type != nil {
		p.parseError(fmt.Sprintf("second TYPE line for metric name %q, or TYPE reported after samples", p.currentMF.GetName()))
		return nil
	}
	// Rest of line is the type.
	if p.readTokenUntilNewline(false); p.err != nil {
		return nil // Unexpected end of input.
	}

	// if the type is unsupported type (now only info and stateset),
	// use untyped to instead
	if _, ok := UnsupportMetricType[strings.ToUpper(p.currentToken.String())]; ok {
		p.currentMF.Type = dto.MetricType_UNTYPED.Enum()
	} else {
		if strings.ToUpper(p.currentToken.String()) == "GAUGEHISTOGRAM" {
			p.currentMF.Type = dto.MetricType_GAUGE_HISTOGRAM.Enum()
			return p.startOfLine
		}
		metricType, ok := dto.MetricType_value[strings.ToUpper(p.currentToken.String())]
		if !ok {
			p.parseError(fmt.Sprintf("unknown metric type %q", p.currentToken.String()))
			return nil
		}
		p.currentMF.Type = dto.MetricType(metricType).Enum()
	}
	return p.startOfLine
}

func (p *OpenMetricsParser) readingUnit() stateFn {
	if p.currentMF.Unit != nil {
		p.parseError(fmt.Sprintf("second UNIT line for metric name %q", p.currentMF.GetUnit()))
		return nil
	}
	if p.readTokenUntilNewline(true); p.err != nil {
		return nil
	}
	if !strings.HasSuffix(p.currentMF.GetName(), p.currentToken.String()) {
		p.parseError(fmt.Sprintf("expected unit as metric name suffix, found metric %q", p.currentMF.GetName()))
		return nil
	}
	p.currentMF.Unit = proto.String(p.currentToken.String())
	return p.startOfLine
}

// parseError sets p.err to a ParseError at the current line with the given
// message.
func (p *OpenMetricsParser) parseError(msg string) {
	p.err = ParseError{
		Line:   p.lineCount,
		Msg:    msg,
		Format: FormatOpenMetrics,
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
func (p *OpenMetricsParser) readTokenUntilWhitespace() {
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

func (p *OpenMetricsParser) setOrCreateCurrentMF() {
	p.currentIsEOF = false

	name := p.currentToken.String()
	if p.currentMF = p.metricFamiliesByName[name]; p.currentMF != nil {
		return
	}
	p.currentMF = &dto.MetricFamily{Name: proto.String(name)}
	p.metricFamiliesByName[name] = p.currentMF
}

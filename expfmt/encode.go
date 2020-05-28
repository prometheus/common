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
	"net/http"

	"github.com/golang/protobuf/proto"
	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	"github.com/prometheus/common/internal/bitbucket.org/ww/goautoneg"

	dto "github.com/prometheus/client_model/go"
)

// Encoder types encode metric families into an underlying wire protocol.
type Encoder interface {
	Encode(*dto.MetricFamily) error
}

// Closer is implemented by Encoders that need to be closed to finalize
// encoding. (For example, OpenMetrics needs a final `# EOF` line.)
//
// Note that all Encoder implementations returned from this package implement
// Closer, too, even if the Close call is a no-op. This happens in preparation
// for adding a Close method to the Encoder interface directly in a (mildly
// breaking) release in the future.
type Closer interface {
	Close() error
}

// EncoderCreator is able to create Encoder based on provided options.
// EncoderCreator can negotiate the Format if HTTP Header is provided.
// Then, using the Format caller may use EncoderCreator to create
// particular Encoder.
type EncoderCreator struct {
	encoders []EncoderImplementation
}

// EncoderImplementation has all of the information required to define Encoder.
type EncoderImplementation struct {
	// HeaderAcceptType defines for what 'Accept' header value this encoder should be used.
	// Example: value 'application/pdf' would correspond to Header "Accept: application/pdf".
	HeaderAcceptType string
	// HeaderAcceptVersion defines for what version of 'Accept' header value this encoder should be used.
	// Example: value '1.0.0' would correspond to Header "Accept: type; version=1.0.0".
	HeaderAcceptVersion string
	// EncodeFormat name of the encoder format. It's actually used as Encoder identifier.
	// Negotiate returns the Format which can be used by NewEncoder to create an Encoder.
	EncodeFormat Format
	// EncoderWriterFunc allows to define the encoding process.
	EncodeWriterFunc func(w io.Writer) func(v *dto.MetricFamily) error
	// CloseFunc allows to define how to close the encoding process.
	CloseFunc func(w io.Writer) func() error
}

type encoderCloser struct {
	encode func(*dto.MetricFamily) error
	close  func() error
}

func (ec encoderCloser) Encode(v *dto.MetricFamily) error {
	return ec.encode(v)
}

func (ec encoderCloser) Close() error {
	return ec.close()
}

var (
	defaultEncoderCreator = NewEncoderCreator()
)

// NewEncoderCreator creates EncoderCreator with default encoders implementations and additional ones that
// are provided as arguments.
func NewEncoderCreator(additionalEncoders ...EncoderImplementation) *EncoderCreator {
	encoders := []EncoderImplementation{
		{
			EncodeFormat: FmtProtoDelim,
			EncodeWriterFunc: func(w io.Writer) func(v *dto.MetricFamily) error {
				return func(v *dto.MetricFamily) error {
					_, err := pbutil.WriteDelimited(w, v)
					return err
				}
			},
		},
		{
			EncodeFormat: FmtProtoCompact,
			EncodeWriterFunc: func(w io.Writer) func(v *dto.MetricFamily) error {
				return func(v *dto.MetricFamily) error {
					_, err := fmt.Fprintln(w, v.String())
					return err
				}
			},
		},
		{
			EncodeFormat: FmtProtoText,
			EncodeWriterFunc: func(w io.Writer) func(v *dto.MetricFamily) error {
				return func(v *dto.MetricFamily) error {
					_, err := fmt.Fprintln(w, proto.MarshalTextString(v))
					return err
				}
			},
		},
		{
			EncodeFormat: FmtText,
			EncodeWriterFunc: func(w io.Writer) func(v *dto.MetricFamily) error {
				return func(v *dto.MetricFamily) error {
					_, err := MetricFamilyToText(w, v)
					return err
				}
			},
		},
		{
			EncodeFormat: FmtOpenMetrics,
			EncodeWriterFunc: func(w io.Writer) func(v *dto.MetricFamily) error {
				return func(v *dto.MetricFamily) error {
					_, err := MetricFamilyToOpenMetrics(w, v)
					return err
				}
			},
			CloseFunc: func(w io.Writer) func() error {
				return func() error {
					_, err := FinalizeOpenMetrics(w)
					return err
				}
			},
		},
	}
	encoders = append(encoders, additionalEncoders...)

	defaultCloseFunc := func(w io.Writer) func() error { return func() error { return nil } }
	for i := range encoders {
		if encoders[i].CloseFunc == nil {
			encoders[i].CloseFunc = defaultCloseFunc
		}
	}

	return &EncoderCreator{encoders: encoders}
}

// Negotiate returns the Content-Type based on the given Accept header. If no
// appropriate accepted type is found, FmtText is returned (which is the
// Prometheus text format). This function will never negotiate FmtOpenMetrics,
// as the support is still experimental. To include the option to negotiate
// FmtOpenMetrics, use NegotiateOpenMetrics.
func (ec *EncoderCreator) Negotiate(h http.Header) Format {
	return ec.negotiate(h, false)
}

// NegotiateIncludingOpenMetrics works like Negotiate but includes
// FmtOpenMetrics as an option for the result. Note that this function is
// temporary and will disappear once FmtOpenMetrics is fully supported and as
// such may be negotiated by the normal Negotiate function.
func (ec *EncoderCreator) NegotiateIncludingOpenMetrics(h http.Header) Format {
	return ec.negotiate(h, true)
}

func (ec *EncoderCreator) negotiate(h http.Header, openMetricsEnabled bool) Format {
	for _, ac := range goautoneg.ParseAccept(h.Get(hdrAccept)) {
		acceptHeader := ac.Type + "/" + ac.SubType
		ver := ac.Params["version"]

		if acceptHeader == ProtoType && ac.Params["proto"] == ProtoProtocol {
			switch ac.Params["encoding"] {
			case "delimited":
				return FmtProtoDelim
			case "text":
				return FmtProtoText
			case "compact-text":
				return FmtProtoCompact
			}
		}
		if ac.Type == "text" && ac.SubType == "plain" && (ver == TextVersion || ver == "") {
			return FmtText
		}
		if openMetricsEnabled {
			if ac.Type+"/"+ac.SubType == OpenMetricsType && (ver == OpenMetricsVersion || ver == "") {
				return FmtOpenMetrics
			}
		}

		for _, item := range ec.encoders {
			if acceptHeader == item.HeaderAcceptType {
				if ver != "" && ver != item.HeaderAcceptVersion {
					continue
				}
				return item.EncodeFormat
			}
		}
	}
	return FmtText
}

// NewEncoder returns a new encoder based on content type negotiation. All
// Encoder implementations returned by NewEncoder also implement Closer, and
// callers should always call the Close method. It is currently only required
// for FmtOpenMetrics, but a future (breaking) release will add the Close method
// to the Encoder interface directly. The current version of the Encoder
// interface is kept for backwards compatibility.
func (ec *EncoderCreator) NewEncoder(w io.Writer, format Format) Encoder {
	for _, encoder := range ec.encoders {
		if format == encoder.EncodeFormat {
			return encoderCloser{
				encode: encoder.EncodeWriterFunc(w),
				close:  encoder.CloseFunc(w),
			}
		}
	}
	panic(fmt.Errorf("expfmt.NewEncoder: unknown format %q", format))
}

// Negotiate returns the Content-Type based on the given Accept header.
// See EncoderCreator.Negotiate for more information.
func Negotiate(h http.Header) Format {
	return defaultEncoderCreator.Negotiate(h)
}

// NegotiateIncludingOpenMetrics works like Negotiate but includes
// FmtOpenMetrics as an option for the result. See
// EncoderCreator.NegotiateIncludingOpenMetrics for more information.
func NegotiateIncludingOpenMetrics(h http.Header) Format {
	return defaultEncoderCreator.NegotiateIncludingOpenMetrics(h)
}

// NewEncoder returns a new encoder based on content type negotiation.
// See EncoderCreator.NewEncoder for more information.
func NewEncoder(w io.Writer, format Format) Encoder {
	return defaultEncoderCreator.NewEncoder(w, format)
}

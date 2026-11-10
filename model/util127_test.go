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

//go:build go1.27

package model

import (
	jsonv1 "encoding/json"
	jsonv2 "encoding/json/v2"
	"reflect"
	"testing"
)

func testV1V2Marshal(t *testing.T, name string, v jsonv1.Marshaler) {
	t.Run("v1v2_"+name, func(t *testing.T) {
		b1, err := v.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		b2, err := jsonv2.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if string(b1) != string(b2) {
			t.Logf("v1=%s", string(b1))
			t.Logf("v2=%s", string(b2))
			t.Fatalf("v1/v2 mismatch")
		}
	})
}

func testRoundTrip(t *testing.T, name string, v any) {
	t.Run("roundtrip_"+name, func(t *testing.T) {
		b, err := jsonv1.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		t.Log(string(b))

		outPtr := reflect.New(reflect.TypeOf(v)).Interface()
		if err := jsonv1.Unmarshal(b, outPtr); err != nil {
			t.Fatal(err)
		}

		out := reflect.ValueOf(outPtr).Elem().Interface()
		if !reflect.DeepEqual(v, out) {
			t.Fatalf("did not round-trip\nwant: %#v\ngot:  %#v", v, out)
		}
	})
}

type unmarshaler interface {
	UnmarshalJSON([]byte) error
}

func testDecodeSuccessParity[T unmarshaler](t *testing.T, input string, newObj func() T) {
	t.Run("decode_success_"+input, func(t *testing.T) {
		s1 := newObj()
		v1err := s1.UnmarshalJSON([]byte(input))

		s2 := newObj()
		v2err := jsonv2.Unmarshal([]byte(input), s2, jsonv1.DefaultOptionsV1()) // matches how decoding works when called from encoding/json

		if v1err != nil {
			t.Fatalf("expected success, got error from v1: %v", v1err)
		}
		if v2err != nil {
			t.Fatalf("expected success, got error from v2: %v", v2err)
		}
		if !reflect.DeepEqual(s1, s2) {
			t.Errorf("inconsistent result: v1: %#v, v2=%#v", s1, s2)
		}
	})
}

func testDecodeErrorParity[T unmarshaler](t *testing.T, input string, newObj func() T) {
	t.Run("decode_error_"+input, func(t *testing.T) {
		s1 := newObj()
		v1err := s1.UnmarshalJSON([]byte(input))
		if v1err == nil {
			t.Fatalf("expected error, got none from v1")
		}

		s2 := newObj()
		v2err := jsonv2.Unmarshal([]byte(input), s2)
		if v2err == nil {
			t.Fatalf("expected error, got none from v2")
		}
	})
}

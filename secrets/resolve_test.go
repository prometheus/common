// Copyright 2025 The Prometheus Authors
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

package secrets

import (
	"reflect"
	"strings"
	"testing"
)

type TestCase struct {
	name        string
	input       interface{}
	want        map[string]string
	errContains string
}

func newSF(secret string) SecretField {
	return SecretField{
		provider: &InlineProvider{
			secret: secret,
		},
	}
}

func newSFRef(secret string) *SecretField {
	val := newSF(secret)
	return &val
}

func normalizeSecretPaths(sp secretPaths) map[string]string {
	normalized := make(map[string]string)
	for path, ptr := range sp {
		normalized[path] = ptr.provider.(*InlineProvider).secret
	}
	return normalized
}

type SimpleStruct struct {
	Secret SecretField
	Day    string
}

type ManyStruct struct {
	Birthday          SecretField
	MothersMaidenName **SecretField
	FavoriteColors    []SecretField
	BookReviews       map[string]*SecretField
	FavoriteMaterial  SecretField
}

type NestedStruct struct {
	Nested    SimpleStruct
	TopSecret SecretField
}

type NestedInterfaceStruct struct {
	NestedInterface interface{}
	TopSecretI      SecretField
}

type PtrNestedStruct struct {
	Indirect *NestedStruct
	Number   int
}

type PrivateField struct {
	Exported SecretField
	secret   SecretField
}
type PrivateNestedField struct {
	Nested PrivateField
}

type DeeplyNestedStruct struct {
	S SecretField
	D *DeeplyNestedStruct
}

func TestGetSecretFields(t *testing.T) {
	pointer := newSF("pointer")

	tests := []TestCase{
		{
			name:        "Direct SecretField",
			input:       newSF("direct"),
			errContains: "not addressable",
		},
		{
			name:  "Plain SecretField",
			input: &pointer,
			want: map[string]string{
				"": "pointer",
			},
		},
		{
			name: "Simple struct with one SecretField",
			input: &SimpleStruct{
				Secret: newSF("secret"),
				Day:    "Monday",
			},
			want: map[string]string{
				"Secret": "secret",
			},
		},
		{
			name: "Struct with multiple SecretFields and nested pointers",
			input: &ManyStruct{
				Birthday: newSF("happy_birthday"),
				MothersMaidenName: func() **SecretField {
					s := newSF("maiden_name")
					p := &s
					return &p
				}(),
				FavoriteColors: []SecretField{
					newSF("red"),
					newSF("blue"),
					newSF("green"),
				},
				BookReviews: map[string]*SecretField{
					"The Hitchhiker's Guide to the Galaxy": newSFRef("hitchhiker_secret"),
					"The Great Gatsby":                     newSFRef("gatsby_secret"),
				},
				FavoriteMaterial: newSF("oak"),
			},
			want: map[string]string{
				"Birthday":                       "happy_birthday",
				"MothersMaidenName":              "maiden_name",
				"FavoriteColors.[0]":             "red",
				"FavoriteColors.[1]":             "blue",
				"FavoriteColors.[2]":             "green",
				"BookReviews.[The Great Gatsby]": "gatsby_secret",
				"BookReviews.[The Hitchhiker's Guide to the Galaxy]": "hitchhiker_secret",
				"FavoriteMaterial": "oak",
			},
		},
		{
			name: "Nested struct with SecretFields at different levels",
			input: &NestedStruct{
				Nested: SimpleStruct{
					Secret: newSF("inner_secret"),
					Day:    "Tuesday",
				},
				TopSecret: newSF("outer_secret"),
			},
			want: map[string]string{
				"Nested.Secret": "inner_secret",
				"TopSecret":     "outer_secret",
			},
		},
		{
			name: "Struct with nil pointer to nested struct",
			input: &PtrNestedStruct{
				Indirect: nil,
				Number:   10,
			},
			want: map[string]string{},
		},
		{
			name: "Struct with populated pointer to nested struct",
			input: &PtrNestedStruct{
				Indirect: &NestedStruct{
					Nested: SimpleStruct{
						Secret: newSF("pointed_inner_secret"),
						Day:    "Wednesday",
					},
					TopSecret: newSF("pointed_outer_secret"),
				},
				Number: 20,
			},
			want: map[string]string{
				"Indirect.Nested.Secret": "pointed_inner_secret",
				"Indirect.TopSecret":     "pointed_outer_secret",
			},
		},
		{
			name: "Struct with private secret field",
			input: &PrivateField{
				Exported: newSF("exported_secret"),
				secret:   newSF("unexported_secret"),
			},
			want: map[string]string{
				"Exported": "exported_secret",
			},
		},
		{
			name: "Nested struct with private secret field",
			input: &PrivateNestedField{
				Nested: PrivateField{
					Exported: newSF("exported_secret"),
					secret:   newSF("unexported_secret"),
				},
			},
			want: map[string]string{
				"Nested.Exported": "exported_secret",
			},
		},
		{
			name:  "Nil input",
			input: nil,
			want:  map[string]string{},
		},
		{
			name: "Pointer to nil input",
			input: func() *int {
				var x *int
				return x
			}(),
			want: map[string]string{},
		},
		{
			name:  "Empty struct",
			input: &struct{}{},
			want:  map[string]string{},
		},
		{
			name: "Struct with no SecretFields",
			input: &struct {
				Name string
				Age  int
			}{
				Name: "John Doe",
				Age:  30,
			},
			want: map[string]string{},
		},
		{
			name: "Deeply nested struct (should handle depth)",
			input: &DeeplyNestedStruct{
				S: newSF("level_1"),
				D: &DeeplyNestedStruct{
					S: newSF("level_2"),
					D: &DeeplyNestedStruct{
						S: newSF("level_3"),
						D: nil,
					},
				},
			},
			want: map[string]string{
				"S":     "level_1",
				"D.S":   "level_2",
				"D.D.S": "level_3",
			},
		},
		{
			name: "Interface holding a SimpleStruct",
			input: &NestedInterfaceStruct{
				NestedInterface: &SimpleStruct{
					Secret: newSF("interface_secret"),
					Day:    "Friday",
				},
				TopSecretI: newSF("interface_top_secret"),
			},
			want: map[string]string{
				"NestedInterface.Secret": "interface_secret",
				"TopSecretI":             "interface_top_secret",
			},
		},
		{
			name: "Interface holding a pointer to SimpleStruct",
			input: &NestedInterfaceStruct{
				NestedInterface: &SimpleStruct{
					Secret: newSF("interface_ptr_secret"),
					Day:    "Saturday",
				},
				TopSecretI: newSF("interface_ptr_top_secret"),
			},
			want: map[string]string{
				"NestedInterface.Secret": "interface_ptr_secret",
				"TopSecretI":             "interface_ptr_top_secret",
			},
		},
		{
			name: "Interface holding a primitive type (no secrets)",
			input: &NestedInterfaceStruct{
				NestedInterface: "hello world",
				TopSecretI:      newSF("primitive_interface_top_secret"),
			},
			want: map[string]string{
				"TopSecretI": "primitive_interface_top_secret",
			},
		},
		{
			name: "Slice of SecretFields",
			input: &[]SecretField{
				newSF("slice_secret_1"),
				newSF("slice_secret_2"),
			},
			want: map[string]string{
				"[0]": "slice_secret_1",
				"[1]": "slice_secret_2",
			},
		},
		{
			name: "Map with SecretField values",
			input: &map[string]SecretField{
				"key1": newSF("map_secret_1"),
				"key2": newSF("map_secret_2"),
			},
			errContains: "not addressable",
		},
		{
			name: "Map with SecretField values references",
			input: &map[string]*SecretField{
				"key1": newSFRef("map_secret_1"),
				"key2": newSFRef("map_secret_2"),
			},
			want: map[string]string{
				"[key1]": "map_secret_1",
				"[key2]": "map_secret_2",
			},
		},
		{
			name:  "Empty slice",
			input: &[]SimpleStruct{},
			want:  map[string]string{},
		},
		{
			name:  "Empty map",
			input: &map[string]SimpleStruct{},
			want:  map[string]string{},
		},
		{
			name: "Deeply nested struct exceeding max depth (should return error)",
			input: func() *DeeplyNestedStruct {
				head := &DeeplyNestedStruct{S: newSF("head_secret")}
				current := head
				for i := 0; i < 51; i++ { // Create a chain longer than 50
					current.D = &DeeplyNestedStruct{S: newSF("level_" + (string)(rune('a'+i)))}
					current = current.D
				}
				return head
			}(),
			errContains: "path traversal exceeded maximum depth",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotPaths, gotErr := getSecretFields(tc.input)

			// Check for expected error.
			if tc.errContains != "" {
				if gotErr == nil {
					t.Fatalf("Expected error containing '%s', but got no error", tc.errContains)
				}
				if !strings.Contains(gotErr.Error(), tc.errContains) {
					t.Errorf("Expected error containing '%s', but got: %v", tc.errContains, gotErr)
				}
				return
			} else if gotErr != nil {
				t.Fatalf("Did not expect an error, but got: %v", gotErr)
			}

			normalizedGotPaths := normalizeSecretPaths(gotPaths)

			if !reflect.DeepEqual(normalizedGotPaths, tc.want) {
				t.Errorf("GetSecretFields() got = %v, want %v", normalizedGotPaths, tc.want)
				for k, v := range tc.want {
					if actualVal, ok := normalizedGotPaths[k]; !ok {
						t.Errorf("Missing %v = %q", k, v)
					} else if actualVal != v {
						t.Errorf("Mimatch %v = %q (want %q)", k, actualVal, v)
					}
				}
				for k, v := range normalizedGotPaths {
					if _, ok := tc.want[k]; !ok {
						t.Errorf("Unexpected %v = %q", k, v)
					}
				}
			}
		})
	}
}

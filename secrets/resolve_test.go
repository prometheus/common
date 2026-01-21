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
	"fmt"
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

func newSF(secret string) Field {
	return Field{
		state: &fieldState{
			providerName: "inline",
			config: &InlineProviderConfig{
				secret: secret,
			},
		},
	}
}

func newSFRef(secret string) *Field {
	val := newSF(secret)
	return &val
}

func normalizeSecretPaths(sp fieldResults[*Field]) map[string]string {
	normalized := make(map[string]string)
	for ptr, path := range sp.paths {
		normalized[path] = ptr.state.config.(*InlineProviderConfig).secret
	}
	return normalized
}

type SimpleStruct struct {
	Secret Field
	Day    string
}

type PointerStruct struct {
	Secret *Field
}

type ManyStruct struct {
	Birthday          Field
	MothersMaidenName **Field
	FavoriteColors    []Field
	BookReviews       map[string]*Field
	FavoriteMaterial  Field
}

type NestedStruct struct {
	Nested    SimpleStruct
	TopSecret Field
}

type NestedInterfaceStruct struct {
	NestedInterface interface{}
	TopSecretI      Field
}

type PointerNestedStruct struct {
	Indirect *NestedStruct
	Number   int
}

type PrivateField struct {
	Exported Field
	secret   Field
}
type PrivateNestedField struct {
	Nested PrivateField
}

type DeeplyNestedStruct struct {
	S Field
	D *DeeplyNestedStruct
}

func TestGetSecretFields(t *testing.T) {
	pointer := newSF("pointer")

	tests := []TestCase{
		{
			name:        "Direct SecretField",
			input:       newSF("direct"),
			errContains: "expected root to be pointer",
		},
		{
			name:  "Plain SecretField",
			input: &pointer,
			want: map[string]string{
				"Field": "pointer",
			},
		},
		{
			name:  "Pointer to SecretField",
			input: &PointerStruct{Secret: &pointer},
			want: map[string]string{
				"PointerStruct.Secret": "pointer",
			},
		},
		{
			name:  "Nil pointer to SecretField",
			input: &PointerStruct{Secret: nil},
			want:  map[string]string{},
		},
		{
			name: "Simple struct with one SecretField",
			input: &SimpleStruct{
				Secret: newSF("secret"),
				Day:    "Monday",
			},
			want: map[string]string{
				"SimpleStruct.Secret": "secret",
			},
		},
		{
			name: "Struct with multiple SecretFields and nested pointers",
			input: &ManyStruct{
				Birthday: newSF("happy_birthday"),
				MothersMaidenName: func() **Field {
					s := newSF("maiden_name")
					p := &s
					return &p
				}(),
				FavoriteColors: []Field{
					newSF("red"),
					newSF("blue"),
					newSF("green"),
				},
				BookReviews: map[string]*Field{
					"The Hitchhiker's Guide to the Galaxy": newSFRef("hitchhiker_secret"),
					"The Great Gatsby":                     newSFRef("gatsby_secret"),
				},
				FavoriteMaterial: newSF("oak"),
			},
			want: map[string]string{
				"ManyStruct.Birthday":                                          "happy_birthday",
				"ManyStruct.MothersMaidenName":                                 "maiden_name",
				"ManyStruct.FavoriteColors[0]":                                 "red",
				"ManyStruct.FavoriteColors[1]":                                 "blue",
				"ManyStruct.FavoriteColors[2]":                                 "green",
				"ManyStruct.BookReviews[The Great Gatsby]":                     "gatsby_secret",
				"ManyStruct.BookReviews[The Hitchhiker's Guide to the Galaxy]": "hitchhiker_secret",
				"ManyStruct.FavoriteMaterial":                                  "oak",
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
				"NestedStruct.Nested.Secret": "inner_secret",
				"NestedStruct.TopSecret":     "outer_secret",
			},
		},
		{
			name: "Struct with nil pointer to nested struct",
			input: &PointerNestedStruct{
				Indirect: nil,
				Number:   10,
			},
			want: map[string]string{},
		},
		{
			name: "Struct with populated pointer to nested struct",
			input: &PointerNestedStruct{
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
				"PointerNestedStruct.Indirect.Nested.Secret": "pointed_inner_secret",
				"PointerNestedStruct.Indirect.TopSecret":     "pointed_outer_secret",
			},
		},
		{
			name: "Struct with private secret field",
			input: &PrivateField{
				Exported: newSF("exported_secret"),
				secret:   newSF("unexported_secret"),
			},
			want: map[string]string{
				"PrivateField.Exported": "exported_secret",
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
				"PrivateNestedField.Nested.Exported": "exported_secret",
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
				"DeeplyNestedStruct.S":     "level_1",
				"DeeplyNestedStruct.D.S":   "level_2",
				"DeeplyNestedStruct.D.D.S": "level_3",
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
				"NestedInterfaceStruct.NestedInterface.Secret": "interface_secret",
				"NestedInterfaceStruct.TopSecretI":             "interface_top_secret",
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
				"NestedInterfaceStruct.NestedInterface.Secret": "interface_ptr_secret",
				"NestedInterfaceStruct.TopSecretI":             "interface_ptr_top_secret",
			},
		},
		{
			name: "Interface holding a primitive type (no secrets)",
			input: &NestedInterfaceStruct{
				NestedInterface: "hello world",
				TopSecretI:      newSF("primitive_interface_top_secret"),
			},
			want: map[string]string{
				"NestedInterfaceStruct.TopSecretI": "primitive_interface_top_secret",
			},
		},
		{
			name: "Slice of SecretFields",
			input: &[]Field{
				newSF("slice_secret_1"),
				newSF("slice_secret_2"),
			},
			want: map[string]string{
				"slice[0]": "slice_secret_1",
				"slice[1]": "slice_secret_2",
			},
		},
		{
			name: "Map with SecretField values",
			input: &map[string]Field{
				"key1": newSF("map_secret_1"),
				"key2": newSF("map_secret_2"),
			},
			want: map[string]string{},
		},
		{
			name: "Map with SecretField key references",
			input: &map[*Field]string{
				newSFRef("map_secret_1"): "val1",
			},
			want: map[string]string{
				"map.Keys()[0]": "map_secret_1",
			},
		},
		{
			name: "Map with SecretField values references",
			input: &map[string]*Field{
				"key1": newSFRef("map_secret_1"),
				"key2": newSFRef("map_secret_2"),
			},
			want: map[string]string{
				"map[key1]": "map_secret_1",
				"map[key2]": "map_secret_2",
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
					current.D = &DeeplyNestedStruct{S: newSF(fmt.Sprintf("level_%d", i))}
					current = current.D
				}
				return head
			}(),
			errContains: "path traversal exceeded maximum depth",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotPaths, gotErr := findFields[*Field](tc.input)

			// Check for expected error.
			if tc.errContains != "" {
				if gotErr == nil {
					t.Fatalf("Expected error containing '%s', but got no error", tc.errContains)
				}
				if !strings.Contains(gotErr.Error(), tc.errContains) {
					t.Errorf("Expected error containing '%s', but got: %v", tc.errContains, gotErr)
				}
				return
			}

			if gotErr != nil {
				t.Fatalf("Did not expect an error, but got: %v", gotErr)
			}

			normalizedGotPaths := normalizeSecretPaths(gotPaths)

			if !reflect.DeepEqual(normalizedGotPaths, tc.want) {
				t.Errorf("GetSecretFields() got = %v, want %v", normalizedGotPaths, tc.want)
				for k, v := range tc.want {
					if actualVal, ok := normalizedGotPaths[k]; !ok {
						t.Errorf("Missing %v = %q", k, v)
					} else if actualVal != v {
						t.Errorf("Mismatch %v = %q (want %q)", k, actualVal, v)
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

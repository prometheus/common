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
package otlptranslator

import "testing"

var labelBenchmarkInputs = []string{
	"",
	"label:with:colons",
	"LabelWithCapitalLetters",
	"label!with&special$chars)",
	"label_with_foreign_characters_字符",
	"label.with.dots",
	"123label",
	"_label_starting_with_underscore",
	"__label_starting_with_2underscores",
}

func BenchmarkNormalizeLabel(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, input := range labelBenchmarkInputs {
			NormalizeLabel(input)
		}
	}
}

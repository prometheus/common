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

//go:build !localvalidationscheme

package model

import "testing"

func testLabelNameIsValid(t *testing.T, labelName LabelName, legacyValid, utf8Valid bool) {
	t.Helper()

	origScheme := NameValidationScheme
	t.Cleanup(func() {
		NameValidationScheme = origScheme
	})

	NameValidationScheme = LegacyValidation
	if labelName.IsValid() != legacyValid {
		t.Errorf("Expected %v for %q using legacy IsValid method", legacyValid, labelName)
	}
	if LabelNameRE.MatchString(string(labelName)) != legacyValid {
		t.Errorf("Expected %v for %q using legacy regexp match", legacyValid, labelName)
	}
	NameValidationScheme = UTF8Validation
	if labelName.IsValid() != utf8Valid {
		t.Errorf("Expected %v for %q using UTF-8 IsValid method", legacyValid, labelName)
	}
}

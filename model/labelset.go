// Copyright 2013 The Prometheus Authors
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
	"fmt"
	"sort"
	"strings"
)

// A LabelSet is a collection of LabelName and LabelValue pairs.  The LabelSet
// may be fully-qualified down to the point where it may resolve to a single
// Metric in the data store or not.  All operations that occur within the realm
// of a LabelSet can emit a vector of Metric entities to which the LabelSet may
// match.
type LabelSet map[LabelName]LabelValue

// Validate checks whether all names and values in the label set
// are valid.
func (ls LabelSet) Validate() error {
	for ln, lv := range ls {
		if !ln.IsValid() {
			return fmt.Errorf("invalid name %q", ln)
		}
		if !lv.IsValid() {
			return fmt.Errorf("invalid value %q", lv)
		}
	}
	return nil
}

// Equal returns true iff both label sets have exactly the same key/value pairs.
func (ls LabelSet) Equal(o LabelSet) bool {
	if len(ls) != len(o) {
		return false
	}
	for ln, lv := range ls {
		olv, ok := o[ln]
		if !ok {
			return false
		}
		if olv != lv {
			return false
		}
	}
	return true
}

// Before compares the metrics, using the following criteria:
//
// If m has fewer labels than o, it is before o. If it has more, it is not.
//
// If the number of labels is the same, the superset of all label names is
// sorted alphanumerically. The first differing label pair found in that order
// determines the outcome: If the label does not exist at all in m, then m is
// before o, and vice versa. Otherwise the label value is compared
// alphanumerically.
//
// If m and o are equal, the method returns false.
func (ls LabelSet) Before(o LabelSet) bool {
	if len(ls) < len(o) {
		return true
	}
	if len(ls) > len(o) {
		return false
	}

	lns := make(LabelNames, 0, len(ls)+len(o))
	for ln := range ls {
		lns = append(lns, ln)
	}
	for ln := range o {
		lns = append(lns, ln)
	}
	// It's probably not worth it to de-dup lns.
	sort.Sort(lns)
	for _, ln := range lns {
		mlv, ok := ls[ln]
		if !ok {
			return true
		}
		olv, ok := o[ln]
		if !ok {
			return false
		}
		if mlv < olv {
			return true
		}
		if mlv > olv {
			return false
		}
	}
	return false
}

// Clone returns a copy of the label set.
func (ls LabelSet) Clone() LabelSet {
	lsn := make(LabelSet, len(ls))
	for ln, lv := range ls {
		lsn[ln] = lv
	}
	return lsn
}

// Merge is a helper function to non-destructively merge two label sets.
func (l LabelSet) Merge(other LabelSet) LabelSet {
	result := make(LabelSet, len(l))

	for k, v := range l {
		result[k] = v
	}

	for k, v := range other {
		result[k] = v
	}

	return result
}

func (l LabelSet) String() string {
	lstrs := make([]string, 0, len(l))
	for l, v := range l {
		lstrs = append(lstrs, fmt.Sprintf("%s=%q", l, v))
	}
	sort.Stable(LabelSorter(lstrs))
	return fmt.Sprintf("{%s}", strings.Join(lstrs, ", "))
}

// Fingerprint returns the LabelSet's fingerprint.
func (ls LabelSet) Fingerprint() Fingerprint {
	return labelSetToFingerprint(ls)
}

// FastFingerprint returns the LabelSet's Fingerprint calculated by a faster hashing
// algorithm, which is, however, more susceptible to hash collisions.
func (ls LabelSet) FastFingerprint() Fingerprint {
	return labelSetToFastFingerprint(ls)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (l *LabelSet) UnmarshalJSON(b []byte) error {
	var m map[LabelName]LabelValue
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	// encoding/json only unmarshals maps of the form map[string]T. It treats
	// LabelName as a string and does not call its UnmarshalJSON method.
	// Thus, we have to replicate the behavior here.
	for ln := range m {
		if !ln.IsValid() {
			return fmt.Errorf("%q is not a valid label name", ln)
		}
	}
	*l = LabelSet(m)
	return nil
}

// LabelSorter implements custom sorting functionality for prometheus LabelSets.
// See: https://github.com/prometheus/common/issues/543
type LabelSorter []string

func (p LabelSorter) Len() int           { return len(p) }
func (p LabelSorter) Less(i, j int) bool { return Less(p[i], p[j]) }
func (p LabelSorter) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func Less(a, b string) bool {
	for len(a) > 0 && len(b) > 0 {
		// get the length of common prefix
		p := lengthOfCommonPrefix(a, b)
		// If there is a common prefix, remove the prefix from both the strings
		if p > 0 {
			a = a[p:]
			b = b[p:]
		}
		if len(a) == 0 {
			return len(b) != 0
		}
		ia := firstNonNumericCharacterIndex(a)
		ib := firstNonNumericCharacterIndex(b)
		switch {
		// If both strings a and b, begin with a numeric character
		case ia > 0 && ib > 0:
			// remove leading zeros if any
			trimmedA, lenTrimmedA := removeLeadingZeros(a[:ia])
			trimmedB, lenTrimmedB := removeLeadingZeros(b[:ib])
			// check the the numeric weightage based on number of digits in the numeric substring
			if lenTrimmedA > lenTrimmedB {
				return false
			} else if lenTrimmedA < lenTrimmedB {
				return true
			}
			// if the length is same, compare them lexicographically
			if trimmedA != trimmedB {
				return trimmedA < trimmedB
			}
			// if the length is same and the value is also same, remove the substring from both the strings
			// and continue with next set of characters
			if ia != len(a) && ib != len(b) {
				a = a[ia:]
				b = b[ib:]
				continue
			}
		// If string a begins with a numeric character and b begins with '=', do not swap
		case ia > 0 && b[0] == '=':
			return false
			// If string b begins with a numeric character and a begins with '=', swap
		case ib > 0 && a[0] == '=':
			return true
		// all the other cases, compare them lexicographically
		default:
			return a < b
		}
	}
	return a < b
}

// lengthOfCommonPrefix, returns a length of common prefix between the given two strings a and b.
// Returns 0 if there is no common prefix
func lengthOfCommonPrefix(a, b string) int {
	i, j := 0, 0
	lenA, lenB := len(a), len(b)
	for i < lenA && j < lenB {
		if a[i] == b[j] {
			i++
			j++
		} else {
			break
		}
	}
	return i
}

// firstNonNumericCharacterIndex returns the index of first non-numeric character
func firstNonNumericCharacterIndex(s string) int {
	for i, c := range s {
		if c < '0' || c > '9' {
			return i
		}
	}
	return len(s)
}

// removeLeadingZeros removes all the leading zeros from the numeric string, and returns it with the new length
func removeLeadingZeros(s string) (string, int) {
	// if the numeric string does not begin with 0, return the same string with its length
	if s[0] != '0' {
		return s, len(s)
	}
	index := 0
	for index < len(s) && s[index] == '0' {
		index++
	}
	return s[index:], len(s[index:])
}

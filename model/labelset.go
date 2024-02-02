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
	"math/big"
	"sort"
	"strconv"
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
	//sort.Strings(lstrs)
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

type LabelSorter []string

func (p LabelSorter) Len() int           { return len(p) }
func (p LabelSorter) Less(i, j int) bool { return Less2(p[i], p[j]) }
func (p LabelSorter) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// Faster, doesn't support sorting of numbers > 64bit unsigned
func Less1(a, b string) bool {
	for len(a) > 0 && len(b) > 0 {
		idx := indexTillCommonPrefix(a, b)
		if idx > 0 {
			a = a[idx:]
			b = b[idx:]
		}
		if len(a) == 0 {
			return len(b) != 0
		}
		ia := indexTillDigit(a)
		ib := indexTillDigit(b)
		if ia > 0 && ib > 0 {
			an, aerr := strconv.ParseUint(a[:ia], 10, 64)
			bn, berr := strconv.ParseUint(b[:ib], 10, 64)
			if aerr != nil && berr != nil {
				if an != bn {
					return an < bn
				}
				if ia != len(a) && ib != len(b) {
					a = a[ia:]
					b = b[ib:]
					continue
				}
			}
		} else if ia > 0 && b[0] == '=' {
			return false
		} else if ib > 0 && a[0] == '=' {
			return true
		}
		return a < b
	}
	return a < b
}

// Slower, and supports sorting of numbers > 64bit unsigned
func Less2(a, b string) bool {
	for len(a) > 0 && len(b) > 0 {
		idx := indexTillCommonPrefix(a, b)
		if idx > 0 {
			a = a[idx:]
			b = b[idx:]
		}
		if len(a) == 0 {
			return len(b) != 0
		}
		ia := indexTillDigit(a)
		ib := indexTillDigit(b)
		if ia > 0 && ib > 0 {
			bigIntA := new(big.Int)
			bigIntB := new(big.Int)
			an, aerr := bigIntA.SetString(a[:ia], 10)
			bn, berr := bigIntB.SetString(b[:ib], 10)
			if aerr && berr {
				if aLessb := an.Cmp(bn); aLessb == -1 {
					return true
				}
				if ia != len(a) && ib != len(b) {
					a = a[ia:]
					b = b[ib:]
					continue
				}
			}
		} else if ia > 0 && b[0] == '=' {
			return false
		} else if ib > 0 && a[0] == '=' {
			return true
		}
		return a < b
	}
	return a < b
}

// indexTillCommonPrefix returns index till the common prefix between the two strings
func indexTillCommonPrefix(a, b string) int {
	m := len(a)
	if n := len(b); n < m {
		m = n
	}
	if m == 0 {
		return 0
	}
	for i := 0; i < m; i++ {
		if (a[i] >= '0' && a[i] <= '9') || (b[i] >= '0' && b[i] <= '9') || (a[i] != b[i]) {
			return i
		}
	}
	return m
}

// indexTillCommonPrefix returns index till the first occurrence of a digit
func indexTillDigit(s string) int {
	for i, c := range s {
		if c < '0' || c > '9' {
			return i
		}
	}
	return len(s)
}

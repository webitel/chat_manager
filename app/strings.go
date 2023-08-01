package app

import (
	"database/sql/driver"
	"strings"
)

// Lower returns given string
// with all Unicode letters
// mapped to their lower case.
var Lower = strings.ToLower

// CaseIgnoreMatch reports whether s and t,
// interpreted as UTF-8 strings,
// are equal under Unicode case-folding.
func CaseIgnoreMatch(s, t string) bool {
	return strings.EqualFold(s, t)
}

func CaseExactMatch(s, t string) bool {
	return (s == t)
}

func CaseExactMatchN(s, t string, n int) bool {
	if n <= 0 {
		return CaseExactMatch(s, t)
	}
	return len(s) >= n && len(t) >= n && CaseExactMatch(s[:n], t[:n])
}

func CaseIgnoreMatchN(s, t string, n int) bool {
	if n <= 0 {
		return CaseIgnoreMatch(s, t)
	}
	return len(s) >= n && len(t) >= n && CaseIgnoreMatch(s[:n], t[:n])
}

// Substrings pattern
type Substrings []string

// Substring MASK defaults
const (
	SubstringAny = '*'
	SubstringOne = '?'
)

func SubstringMask(s string, any, one rune) Substrings {

	if any == 0 {
		any = SubstringAny
	}
	// NOT implemented yet !
	// if one == 0 {
	// 	one = SubstringOne
	// }
	sv := strings.Split(s, string(any))
	// omit any empty sequences: [1:len()-2]
	for i := len(sv) - 2; i > 0; i-- {
		if len(sv[i]) == 0 {
			// cut
			sv = append(sv[:i], sv[i+1:]...)
		}
	}
	return Substrings(sv)
}

func Substring(s string) Substrings {
	return SubstringMask(s, 0, 0)
}

// IsPresent reports whether given string s
// represents 'present' filter assertion value
//
// Shorthand for (s == "*")
func IsPresent(s string) bool {
	return s == string(SubstringAny)
}

// IsPresent reports whether subs represents 'present' filter set
func (subs Substrings) IsPresent() bool {
	// Imit (s == "*")
	// strings.Split("", "*") = []string{""}
	// strings.Split("*", "*") = []string{"", ""}
	for n, part := range subs {
		if n > 1 || part != "" {
			return false
		}
	}
	return true
}

func (subs Substrings) Copy() []string {
	n := len(subs)
	if n == 0 {
		return nil
	}
	sub2 := make([]string, n)
	copy(sub2, subs)
	return sub2
}

func (subs Substrings) String() string {
	return strings.Join(subs, string(SubstringAny))
}

// -- can occur at most once
func (subs Substrings) Initial() (string, bool) {
	// if len(subs) > 1 {
	var pfx string
	if len(subs) != 0 {
		pfx = subs[0]
	}
	return pfx, "" != pfx
}

// -- can occur at most once
func (subs Substrings) Final() (string, bool) {
	var sfx string
	if n := len(subs); n > 1 {
		sfx = subs[n-1]
	}
	return sfx, "" != sfx
}

func (subs Substrings) Any() []string {
	if n := len(subs); n > 2 {
		return subs[1 : n-1] // inner::any
	}
	return nil
}

func (m Substrings) Match(s string) bool {

	if m.IsPresent() {
		return "" != s
	}

	matchText := func(term string, indexFunc func(string) int) bool {
		// matchOne: /(initial:^[^?]*)(any:[^?]*)(final:.*)
		for {
			// SPLIT: ONE (?)
			x := strings.IndexByte(term, '?')
			// cut
			a := x
			if x < 0 {
				a = len(term)
			}

			text := term[:a]
			term = term[a:]

			i := indexFunc(text) // strings.Index(s, part)
			if i < 0 {           // NOT MATCH (!)
				return false
			}
			// MATCH (!) advance
			s = s[i+len(text):]

			if x < 0 {
				// FINAL (!)
				break
			}

			// MATCH: ONE (?) // '?' rune
			if s == "" {
				return false
			}

			s = s[1:] // indexed (!)
			term = term[1:]
		}
		return true
	}

	if prefix, _ := m.Initial(); prefix != "" {
		// MATCH: PREFIX (?)
		index := func(term string) int {
			if term == "" {
				return 0
			}
			if CaseIgnoreMatchN(s, term, len(term)) {
				return 0
			}
			return -1
		}
		// SCAN (!)
		if !matchText(prefix, index) {
			return false
		}
	}

	// TODO: any
	for _, any := range m.Any() {
		// MATCH: CONTAINS (?)
		index := func(term string) int {
			if term == "" {
				return 0
			}
			return strings.Index(
				strings.ToLower(s),
				strings.ToLower(term),
			)
		}
		// SCAN (!)
		if !matchText(any, index) {
			return false
		}
	}

	if suffix, ok := m.Final(); ok {
		// if suffix == "" {
		// 	s = "" // MATCH: ANY(*)
		// } else
		if len(s) >= len(suffix) {
			// MATCH: SUFFIX (?)
			s = s[len(s)-len(suffix):]
			index := func(term string) int {
				if term == "" {
					// MATCH: ONE(?)
					return 0
				}
				if CaseIgnoreMatchN(s, term, len(term)) {
					return 0
				}
				return -1
			}
			// SCAN (!)
			if !matchText(suffix, index) {
				return false
			}
		}
	}
	// MATCH: ALL (?)
	return s == ""
}

func (m Substrings) Value() (driver.Value, error) {
	if len(m) == 0 {
		return "", nil
	}
	// TODO: escape(%)
	v := m.Copy()
	const escape = "\\" // https://postgrespro.ru/docs/postgresql/12/functions-matching#FUNCTIONS-LIKE
	for i := 0; i < len(v); i++ {
		v[i] = strings.ReplaceAll(v[i], "_", escape+"_") // escape control '_' (single char entry)
		v[i] = strings.ReplaceAll(v[i], "?", "_")        // propagate '?' char for PostgreSQL purpose
		v[i] = strings.ReplaceAll(v[i], "%", escape+"%") // escape control '%' (any char(s) or none)
	}
	return strings.Join(v, "%"), nil
}

func SubstringMatch(pattern, value string) bool {
	return SubstringMask(pattern, '*', '?').Match(value)
}

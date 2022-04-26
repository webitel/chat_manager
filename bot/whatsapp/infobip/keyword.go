package infobip

import "strings"

// CaseIgnoreMatch reports whether s and t,
// interpreted as UTF-8 strings,
// are equal under Unicode case-folding.
func caseIgnoreMatch(s, t string) bool {
	return strings.EqualFold(s, t)
}

func caseExactMatch(s, t string) bool {
	return s == t
}

func caseExactMatchN(s, t string, n int) bool {
	if n <= 0 {
		return caseExactMatch(s, t)
	}
	return len(s) >= n && len(t) >= n && caseExactMatch(s[:n], t[:n])
}

func caseIgnoreMatchN(s, t string, n int) bool {
	if n <= 0 {
		return caseIgnoreMatch(s, t)
	}
	return len(s) >= n && len(t) >= n && caseIgnoreMatch(s[:n], t[:n])
}
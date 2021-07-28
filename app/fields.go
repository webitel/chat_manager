package app

import (
	"strings"
	"unicode"
)

// Name canonize s to alphanumeric lower code name
func Name(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	// for _, r := range s {
	// 	switch {
	// 	case 'a' <= r && r <= 'z':
	// 	case '0' <= r && r <= '9':
	// 	case '_' == r:
	// 	default: 
	// 	}
	// }
	return s
}

// HasScope reports whether given scope contains key
func HasScope(scope []string, key string) bool {
	if len(scope) == 0 {
		return false // nothing(!)
	}
	// key = Name(key) // CaseIgnoreMatch(!)
	if len(key) == 0 {
		return true // len(scope) != 0
	}
	i, n := 0, len(scope)
	for ; i < n && scope[i] != key; i++ {
		// break; match found !
	}
	return i < n
}

// AddScope appends UNIQUE(+lower) names to scope
// and returns, optionaly new, scope slice
func AddScope(scope []string, keys ...string) []string {

	var (
		m = len(keys)
		n = len(scope)
	)

	if m == 0 {
		return scope
	}

	if c := n + m; cap(scope) < c {
		grow := make([]string, n, c)
		copy(grow, scope)
		scope = grow
	}
	// var name string
	for _, key := range keys {
		// key = Name(key) // CaseIgnoreMatch(!)
		if len(key) == 0 {
			continue
		}
		// CaseExactMatch(!)
		if !HasScope(scope, key) {
			scope = append(scope, key)
		}
	}
	return scope
}

// FieldsCopy returns copy of unique set
// of given fields, all are in lower case
func FieldsCopy(fields []string) []string {
	// NOTE: in lower case
	return AddScope(nil, fields...)
}

// splitFields reports whether given rune r
// represents fields=a[,b] COMMA|WSP delimiter
// SplitFunc to explode inline fields selector
func splitFields(r rune) bool {
	return ',' == r || unicode.IsSpace(r)
}

// InlineFields explode inline 'attr,attr2 attr3' selector as ['attr','attr2','attr3']
func InlineFields(selector string) []string {
	// split func to explode inline userattrs selector
	selector = strings.ToLower(selector)
	return strings.FieldsFunc(selector, splitFields)
}

// SelectFields acts like InlineFields method
// but maps some, well-known, attributes selector(s):
//
// '*' => application attributes
// '+' => application attributes & system operational
//
func SelectFields(attributes, operational []string) func(string) []string {
	return func(selector string) []string {
		// selector = strings.ToLower(selector)
		fields := strings.FieldsFunc(selector, splitFields)
		if len(fields) == 0 {
			return attributes // imit '*'
		}
		for i := 0; i < len(fields); i++ {

			switch fields[i] {
			case "*":
				
				n := len(fields)
				fields = MergeFields(fields[:i],
					MergeFields(attributes[:len(attributes):len(attributes)], fields[i+1:]))
				// advanced ?
				if len(fields) > n {
					i = len(fields)-n-1
				}

			case "+":
				
				n := len(fields)
				fields = MergeFields(fields[:i], MergeFields(
					MergeFields(operational[:len(operational):len(operational)], attributes),
					fields[i+1:],
				))
				// advanced ?
				if len(fields) > n {
					i = len(fields)-n-1
				}
			}
		}
		return fields
	}
}

// FieldsFunc normalize a selection list src of the attributes to be returned.
//
// 1. An empty list with no attributes requests the return of all user attributes.
// 2. A list containing "*" (with zero or more attribute descriptions)
//    requests the return of all user attributes in addition to other listed (operational) attributes.
//
// e.g.: ['id,name','display'] returns ['id','name','display']
func FieldsFunc(src []string, fn func(string) []string) []string {
	if len(src) == 0 {
		return fn("")
	}

	var dst []string
	for i := 0; i < len(src); i++ {
		// explode single selection attr
		switch set := fn(src[i]); len(set) {
		case 0: // none
			src = append(src[:i], src[i+1:]...)
			i-- // process this i again
		case 1: // one
			if len(set[0]) == 0 {
				src = append(src[:i], src[i+1:]...)
				i-- 
			} else if dst == nil {
				src[i] = set[0]
			} else {
				dst = MergeFields(dst, set)
			}
		default: // many
			// NOTE: should rebuild output
			if dst == nil && i > 0 {
				// copy processed entries
				dst = make([]string, i, len(src)-1+len(set))
				copy(dst, src[:i])
			}
			dst = MergeFields(dst, set)
		}
	}
	if dst == nil {
		return src
	}
	return dst
}

// MergeFields appends unique set from src to dst.
func MergeFields(dst, src []string) []string {
	if len(src) == 0 {
		return dst
	}
	// 
	if cap(dst) - len(dst) < len(src) {
		ext := make([]string, len(dst), len(dst) + len(src))
		copy(ext, dst)
		dst = ext
	}

	next: // append unique set of src to dst
	for _, attr := range src {
		if len(attr) == 0 {
			continue
		}
		// look backwords for duplicates
		for j := len(dst)-1; j >= 0; j-- {
			if strings.EqualFold(dst[j], attr) {
				continue next // duplicate found
			}
		}
		// append unique attr
		dst = append(dst, attr)
	}
	return dst
}


func FieldsMask(paths []string, level int, base ...string) []string {
	const delim = "."
	dst := make([]string, 0, len(paths))
	pattern := strings.Join(base, delim)
	if pattern != "" {
		pattern += delim
	}
	for i := 0; i < len(paths); i++ {
		path := paths[i]
		if len(pattern) > 0 &&
			len(path) < len(pattern) ||
			path[:len(pattern)] != pattern { // starts with {section} prefix
			continue // ignore: does not match base pattern
		}
		// NOTE: pattern is empty or match found !
		path = path[len(pattern):]
		if len(path) == 0 {
			// MATCH: equals base pattern
			continue
		}
		// // Has relative path assigned ?
		// if len(path) > 0 {
		// 	// has relative path
		// 	if !CaseExactMatchN(path, delim, len(delim)) {
		// 		// MATCH: NOT ! expect to be 'delim' first
		// 		continue // ignore: does not match base pattern
		// 	}
		// 	path = path[len(delim):]
		// // NOTE: len(path) == 0
		// } else { continue }
		// // } else if len(pattern) > 0 {
		// // 	// MATCH: equals base pattern
		// // 	// TODO: add as the default match
		// // 	dst = append(dst, pattern)
		// // 	continue
		// // }

		parts := strings.Split(path, delim)
		if len(parts) <= level {
			dst = append(dst, path)
			continue
		}
		// NOTE: need modifications: len(parts) >= level
		path = strings.Join(parts[:level], delim)
		dst = append(dst, path)
		path = pattern + path
		// lookup ahead for the same section prefix
		// NOTE: assume input paths are already sorted
		for i++; i < len(paths); i++ {
			if len(paths[i]) < len(path) || // is longer or equals {section} prefix
				paths[i][:len(path)] != path { // starts with {section} prefix
				break // start of next section
			}
		}
		i-- // suppress all with same {section} prefix
	}
	return dst
}
package micro

import "log/slog"

func cloneFields(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func combineFields(dst, src map[string]any) map[string]any {
	if len(src) == 0 {
		return dst
	}
	if len(dst) == 0 {
		return cloneFields(src)
	}
	for att, vs := range src {
		dst[att] = vs
	}
	return dst
}

func convertAny(fields map[string]any) []any {
	n := len(fields)
	if n == 0 {
		return nil
	}
	var (
		i    int
		heap = make([]slog.Attr, n)
		list = make([]any, 0, n)
	)
	for att, vs := range fields {
		e := &heap[i]
		e.Key = att
		e.Value = convertValue(vs)
		i++
		list = append(list, e)
	}
	return list
}

func convertAttrs(fields map[string]any) []slog.Attr {
	n := len(fields)
	if n == 0 {
		return nil
	}
	var (
		i     int
		attrs = make([]slog.Attr, n)
	)
	for att, vs := range fields {
		e := &attrs[i]
		e.Key = att
		e.Value = convertValue(vs)
		i++
	}
	return attrs
}

func convertValue(v any) slog.Value {
	// AnyValue(v any) Value
	// BoolValue(v bool) Value
	// DurationValue(v time.Duration) Value
	// Float64Value(v float64) Value
	// GroupValue(as ...Attr) Value
	// Int64Value(v int64) Value
	// IntValue(v int) Value
	// StringValue(value string) Value
	// TimeValue(v time.Time) Value
	// Uint64Value(v uint64) Value
	return slog.AnyValue(v)
}

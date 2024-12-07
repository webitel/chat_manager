package log

import "log/slog"

type deferValue struct {
	eval  func() slog.Value
	cache slog.Value
}

var _ slog.LogValuer = (*deferValue)(nil)

// A [LogValuer] is any Go value that can convert itself into a Value for logging.
//
// This mechanism may be used to defer expensive operations until they are needed,
// or to expand a single value into a sequence of components.
func (v *deferValue) LogValue() slog.Value {
	if v.cache.Any() == nil && v.eval != nil {
		v.cache = v.eval()
	}
	return v.cache // nil
}

// [slog.LogValuer] as a function, used to defer evaluation.
// Helpful when you won't know whether Level is enabled before emit.
// In other words: [slog.Value] on demand.
func DeferValue(eval func() slog.Value) slog.LogValuer {
	return &deferValue{eval: eval}
}

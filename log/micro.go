// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Original source: github.com/micro/micro/v3/service/logger/default.go

package log

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	dlog "github.com/micro/micro/v3/service/debug/log"
	log "github.com/micro/micro/v3/service/logger"
)

func init() {
	lvl, err := log.GetLevel(os.Getenv("MICRO_LOG_LEVEL"))
	if err != nil {
		lvl = log.InfoLevel
	}

	log.DefaultLogger = log.NewHelper(
		NewLogger(
			log.WithLevel(lvl),
			WithMultiline(true),
		),
	)
	// log.DefaultLogger = DefaultLogger
}

type optionMultiline struct{}
type optionColorize struct{}
type optionFormat struct{}

// WithColor indicate whether to colorize output within level
func WithMultiline(set bool) log.Option {
	return log.SetOption(optionMultiline{}, set)
}

func isMultiline(opts *log.Options) bool {
	is, _ := opts.Context.Value(optionMultiline{}).(bool)
	return is
}

// // WithColor indicate whether to colorize output within level
// func WithColor(colorize bool) log.Option {
// 	return log.SetOption(optionColorize{}, colorize)
// }

// // WithColor indicate whether to colorize output within level
// func WithFormat(layout string) log.Option {
// 	// $time - timestamp
// 	// $text - message
// 	// $file - caller
// 	// $meta - context metadata fields
// 	return log.SetOption(optionFormat{}, layout)
// }

type defaultLogger struct {
	sync.RWMutex
	opts      log.Options
	Multiline bool
}

// Init(opts...) should only overwrite provided options
func (l *defaultLogger) Init(opts ...log.Option) error {
	for _, o := range opts {
		o(&l.opts)
	}
	l.Multiline = isMultiline(&l.opts)
	return nil
}

func (l *defaultLogger) String() string {
	return "default"
}

func (l *defaultLogger) Fields(fields map[string]interface{}) log.Logger {
	l.Lock()
	l.opts.Fields = copyFields(fields)
	l.Unlock()
	return l
}

func copyFields(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// logCallerfilePath returns a package/file:line description of the caller,
// preserving only the leaf directory name and file name.
func logCallerfilePath(loggingFilePath string) string {
	// To make sure we trim the path correctly on Windows too, we
	// counter-intuitively need to use '/' and *not* os.PathSeparator here,
	// because the path given originates from Go stdlib, specifically
	// runtime.Caller() which (as of Mar/17) returns forward slashes even on
	// Windows.
	//
	// See https://github.com/golang/go/issues/3335
	// and https://github.com/golang/go/issues/18151
	//
	// for discussion on the issue on Go side.
	idx := strings.LastIndexByte(loggingFilePath, '/')
	if idx == -1 {
		return loggingFilePath
	}
	idx = strings.LastIndexByte(loggingFilePath[:idx], '/')
	if idx == -1 {
		return loggingFilePath
	}
	return loggingFilePath[idx+1:]
}

func (l *defaultLogger) Log(level log.Level, v ...interface{}) {
	// TODO decide does we need to write message if log level not used?
	if !l.opts.Level.Enabled(level) {
		return
	}

	l.RLock()
	fields := copyFields(l.opts.Fields)
	l.RUnlock()

	fields["level"] = level.String()

	if _, file, line, ok := runtime.Caller(l.opts.CallerSkipCount); ok {
		fields["file"] = fmt.Sprintf("%s:%d", logCallerfilePath(file), line)
	}

	rec := dlog.Record{
		Timestamp: time.Now(),
		Message:   strings.TrimSpace(fmt.Sprint(v...)),
		Metadata:  make(map[string]string, len(fields)),
	}

	if !l.Multiline {
		rec.Message = strings.ReplaceAll(rec.Message.(string), "\n", "")
	}

	keys := make([]string, 0, len(fields))
	for k, v := range fields {
		keys = append(keys, k)
		rec.Metadata[k] = fmt.Sprintf("%v", v)
	}

	sort.Strings(keys)
	metadata := ""

	for _, k := range keys {
		metadata += fmt.Sprintf(" %s=%v", k, fields[k])
	}

	t := rec.Timestamp.Format("2006-01-02 15:04:05")
	fmt.Fprintf(l.opts.Out, "%s %s %v\n", t, metadata, rec.Message)
}

func (l *defaultLogger) Logf(level log.Level, format string, v ...interface{}) {
	//	 TODO decide does we need to write message if log level not used?
	if level < l.opts.Level {
		return
	}

	l.RLock()
	fields := copyFields(l.opts.Fields)
	l.RUnlock()

	fields["level"] = level.String()

	if _, file, line, ok := runtime.Caller(l.opts.CallerSkipCount); ok {
		fields["file"] = fmt.Sprintf("%s:%d", logCallerfilePath(file), line)
	}

	rec := dlog.Record{
		Timestamp: time.Now(),
		Message:   strings.TrimSpace(fmt.Sprintf(format, v...)),
		Metadata:  make(map[string]string, len(fields)),
	}

	if !l.Multiline {
		rec.Message = strings.ReplaceAll(rec.Message.(string), "\n", "")
	}

	keys := make([]string, 0, len(fields))
	for k, v := range fields {
		keys = append(keys, k)
		rec.Metadata[k] = fmt.Sprintf("%v", v)
	}

	sort.Strings(keys)
	metadata := ""

	for _, k := range keys {
		metadata += fmt.Sprintf(" %s=%v", k, fields[k])
	}

	t := rec.Timestamp.Format("2006-01-02 15:04:05.000000")
	fmt.Fprintf(l.opts.Out, "%s %s %v\n", t, metadata, rec.Message)
}

func (l *defaultLogger) Options() log.Options {
	// not guard against options Context values
	l.RLock()
	opts := l.opts
	opts.Fields = copyFields(l.opts.Fields)
	l.RUnlock()
	return opts
}

// NewLogger builds a new logger based on options
func NewLogger(opts ...log.Option) log.Logger {
	// Default options
	options := log.Options{
		Level:           log.InfoLevel,
		Fields:          make(map[string]interface{}),
		Out:             os.Stderr,
		CallerSkipCount: 2,
		Context:         context.Background(),
	}

	l := &defaultLogger{opts: options}
	if err := l.Init(opts...); err != nil {
		l.Log(log.FatalLevel, err)
	}

	return l
}

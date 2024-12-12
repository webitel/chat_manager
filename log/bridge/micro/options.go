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
// Original source: github.com/micro/micro/v3/service/logger/options.go

package micro

import (
	"context"
	"io"

	mlog "github.com/micro/micro/v3/service/logger"
)

type (
	Option  = mlog.Option
	Options = mlog.Options
)

// WithFields set default fields for the logger
func WithFields(fields map[string]any) Option {
	return func(args *Options) {
		// combine
		if len(fields) == 0 {
			return
		}
		if len(args.Fields) == 0 {
			args.Fields = fields
			return
		}
		for att, value := range fields {
			args.Fields[att] = value
		}
	}
}

// WithLevel set default level for the logger
func WithLevel(level mlog.Level) Option {
	return func(args *Options) {
		args.Level = level
	}
}

// WithOutput set default output writer for the logger
func WithOutput(out io.Writer) Option {
	return func(args *Options) {
		args.Out = out
	}
}

// WithCallerSkipCount set frame count to skip
func WithCallerSkipCount(c int) Option {
	return func(args *Options) {
		args.CallerSkipCount = c
	}
}

func SetOption(key, val interface{}) Option {
	return func(args *Options) {
		if args.Context == nil {
			args.Context = context.Background()
		}
		args.Context = context.WithValue(args.Context, key, val)
	}
}

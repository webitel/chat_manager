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
	"log/slog"
	"os"
	"strings"

	mlog "github.com/micro/micro/v3/service/logger"

	microslog "github.com/webitel/chat_manager/log/bridge/micro"
)

var (
	// $WBTL_LOG_LEVEL
	verbose slog.LevelVar
)

// Verbose returns the log.Level
func Verbose() slog.Level {
	return verbose.Level()
}

func init() {

	// default:
	verbose.Set(
		// -"debug" ; [ +"info" ] ; +"warn" ; +"error"
		slog.LevelInfo,
	)

	setLevel := ""
	for _, envar := range []string{
		"WBTL_LOG_LEVEL",
		"MICRO_LOG_LEVEL", // alias
	} {
		setLevel = strings.TrimSpace(
			os.Getenv(envar),
		)
		if setLevel != "" {
			break // found
		}
	}

	// CUSTOM
	logLevel, err := parseLevel(setLevel)
	if err == nil {
		verbose.Set(logLevel)
	}

	output := os.Stdout

	// log[/slog]
	handler := console(
		output, verbose.Level(),
	)
	slog.SetDefault(
		slog.New(handler),
	)
	// github.com/micro/micro/v3/service/logger
	mlog.DefaultLogger = microslog.New(
		func(conf *mlog.Options) {
			// convert: slog.Level 2 micro.Level
			conf.Level = slog2microLevel(
				verbose.Level(),
			)
		},
	)
}

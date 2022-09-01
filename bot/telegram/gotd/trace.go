package gotd

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/gotd/td/bin"
	"github.com/gotd/td/tdp"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
)

// prettyMiddleware pretty-prints request and response.
func prettyMiddleware(prompt string) telegram.MiddlewareFunc {
	prompt = strings.TrimSpace(prompt)
	if prompt != "" {
		prompt += "\n"
	}
	return func(next tg.Invoker) telegram.InvokeFunc {
		return func(ctx context.Context, input bin.Encoder, output bin.Decoder) error {
			fmt.Printf("%s← %s\n", prompt, formatObject(input))
			start := time.Now()
			err := next.Invoke(ctx, input, output)
			elapsed := time.Since(start).Round(time.Millisecond)
			if err != nil {
				fmt.Printf("%s→ (%s) ERR %v\n", prompt, elapsed, err)
				return err
			}

			fmt.Printf("%s→ (%s) %s\n", prompt, elapsed, formatObject(output))

			return nil
		}
	}
}

func formatObject(input interface{}) string {
	o, ok := input.(tdp.Object)
	if !ok {
		// Handle tg.*Box values.
		rv := reflect.Indirect(reflect.ValueOf(input))
		for i := 0; i < rv.NumField(); i++ {
			if v, ok := rv.Field(i).Interface().(tdp.Object); ok {
				return formatObject(v)
			}
		}
		return fmt.Sprintf("%T (not object)", input)
	}
	return tdp.Format(o)
}

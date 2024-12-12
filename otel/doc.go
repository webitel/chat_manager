// Package you can use as a plugin
// to configure [O]pen[Tel]emetry environment
//
// import (
//
//	"context"
//	"os"
//
//	"webitel.go/service/otel"
//
// )
//
// func main() {
//
//		ctx := context.Background()
//		err := otel.Configure(ctx)
//		if err != nil {
//			os.Exit(1)
//		}
//		defer otel.Shutdown(ctx)
//
//		// your code ...
//	}
package otel

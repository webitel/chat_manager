package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"

	"github.com/micro/micro/v3/plugin"
	"github.com/urfave/cli/v2"
)

// --pprof=[addr] allows you to serve runtime profiling data via separate HTTP server host
var PluginPprof = plugin.NewPlugin(
	plugin.WithName("pprof"),
	plugin.WithFlag(
		&cli.StringFlag{
			Name:  "pprof",
			Usage: "Address to serve runtime profiling data via HTTP (e.g. localhost:6060)",
		},
	),
	plugin.WithInit(func(ctx *cli.Context) error {

		if !ctx.IsSet("pprof") {
			return nil
		}

		addr := ctx.String("pprof")
		if addr == "" {
			return nil
		}

		mux := http.NewServeMux()

		mux.HandleFunc("GET /debug/pprof/", pprof.Index)
		mux.HandleFunc("GET /debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("GET /debug/pprof/trace", pprof.Trace)

		// "/" => "/debug/pprof/"
		mux.Handle("GET /", http.RedirectHandler(
			"/debug/pprof/", http.StatusTemporaryRedirect,
		))

		srv := http.Server{
			Addr:                         addr,
			Handler:                      mux,
			DisableGeneralOptionsHandler: false,
			ReadTimeout:                  0,
			ReadHeaderTimeout:            0,
			WriteTimeout:                 0,
			IdleTimeout:                  0,
			MaxHeaderBytes:               0,
			// TLSConfig:                    &tls.Config{},
			// TLSNextProto:                 map[string]func(*http.Server, *tls.Conn, http.Handler){},
			// ConnState: func(net.Conn, http.ConnState) {
			// 	panic("TODO")
			// },
			// ErrorLog: &log.Logger{},
			// BaseContext: func(net.Listener) context.Context {
			// 	panic("TODO")
			// },
			// ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			// 	panic("TODO")
			// },
			// HTTP2:     &http.HTTP2Config{},
			// Protocols: &http.Protocols{},
		}

		start := make(chan error)
		go func(srv *http.Server) {
			var (
				err error
				args []any
				mlog = slog.Info
			)
			defer func() {
				if err != nil && err != http.ErrServerClosed {
					mlog, args = slog.Error, append(args, slog.Any("error", err))
				}
				mlog("Server [http] Close runtime profiling", args...)
			}()
			
			// err = srv.ListenAndServe()
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				// release caller
				start <- fmt.Errorf("--pprof: %v", err)
				return // err
			}
			addr = ln.Addr().String() // normalize ..
			args = append(args, slog.String("http.host", addr))
			mlog("Server [http] Start runtime profiling", args...)
			start <- nil // release caller
			err = srv.Serve(ln)

		}(&srv)

		// wait for start
		err := <-start
		close(start)
		if err != nil {
			// failed to start http.Server
			return err
		}

		// wrap cmd.Action && finally srv.Close()
		cmd := ctx.Command
		run := cmd.Action
		cmd.Action = func(ctx *cli.Context) (err error) {
			return errors.Join(run(ctx), srv.Close())
		}

		return nil
	}),
)

// CommandPlugin plugin additional logic for given cli.Command
func CommandPlugin(command *cli.Command, plugin plugin.Plugin) {

	command.Flags = append(command.Flags, plugin.Flags()...)

	// cmd.Before = cli.BeforeFunc(failFastAction())
	
	before := command.Before
	command.Before = func(ctx *cli.Context) error {
		err := plugin.Init(ctx)
		if err != nil {
			return err
		}
		if before != nil {
			return before(ctx)
		}
		return nil
	}
}

func failFastAction(chain ...cli.ActionFunc) cli.ActionFunc {
	return func(ctx *cli.Context) (err error) {
		for _, run := range chain {
			if run == nil {
				continue // omit
			}
			err = run(ctx)
			if err != nil {
				return err // failed
			}
		}
		return nil // OK
	}
}

func chainAllAction(chain ...cli.ActionFunc) cli.ActionFunc {
	return func(ctx *cli.Context) (err error) {
		for _, run := range chain {
			if run == nil {
				continue
			}
			err = errors.Join(err, run(ctx))
			// continue
		}
		return err
	}
}
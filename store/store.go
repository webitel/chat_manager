package store

import (
	"context"
	"sync"

	"github.com/urfave/cli/v2"
)

type Driver interface {
	Open(ctx context.Context) error
}

type Connector func(cli *cli.Context) (Driver, error)

var (
	driverMx sync.RWMutex
	drivers  map[string]Connector
)

func Register(name string, ctor Connector) {

	if name == "" {
		panic("store: register connector name is missing")
	}

	if ctor == nil {
		panic("store: register <nil> driver connector")
	}

	driverMx.Lock()
	defer driverMx.Unlock()

	if _, ok := drivers[name]; ok { // && c != ctor {
		panic("store: register duplicate " + name + " connector")
	}

	drivers[name] = ctor
}

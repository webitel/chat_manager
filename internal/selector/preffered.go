package util

import (
	"strings"

	log "github.com/micro/micro/v3/service/logger"

	"github.com/micro/micro/v3/util/selector"
	"github.com/micro/micro/v3/util/selector/random"
	"github.com/micro/micro/v3/util/selector/roundrobin"
)

// Select a route from the pool using the strategy
type Select func(hosts []string, opts ...selector.SelectOption) (selector.Next, error)

var (
	Random     = random.NewSelector()
	RoundRobin = roundrobin.NewSelector()
)

// PrefferedHost returns node
// that matches given addr prefix
// or any available otherwise
func PrefferedHost(addr string) Select {

	if addr == "" {
		return RoundRobin.Select
	}

	return func(hosts []string, opts ...selector.SelectOption) (selector.Next, error) {

		var (
			ok   bool
			node string
		)

		for _, host := range hosts {
			if strings.HasPrefix(host, addr) {
				node, ok = host, true
				break
			} else if strings.HasSuffix(host, addr) {
				node, ok = host, true
				break
			} else if node == "" {
				node = host // default: first available
			}
		}

		// NOT FOUND ! PEEK: FIRST
		if !ok && node != "" {
			log.Warnf(
				"Preffered service host %q not found; peer=%s",
				addr, node,
			)
		}

		if node == "" {
			return nil, selector.ErrNoneAvailable
		}

		// logger.Info().Msg("SELECT NODE")
		return func() string {
			return node
		}, nil
	}
}

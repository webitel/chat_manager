package util

import (
	"fmt"
	"github.com/micro/go-micro/v2/util/log"
	"strings"

	"github.com/micro/go-micro/v2/registry"
	"github.com/micro/go-micro/v2/client/selector"
)

// PrefferedHost returns node
// that matches given addr prefix
// or any available otherwise
func PrefferedHost(addr string) selector.Strategy {
	
	if addr == "" {
		return selector.Random
	}
	
	return func(services []*registry.Service) selector.Next {
				
		var (
			ok bool
			node *registry.Node
		)
		
		lookup:
		for _, service := range services {
			for _, seed := range service.Nodes {
				if strings.HasPrefix(seed.Address, addr) {
					
					node, ok = seed, true
					break lookup

				} else if node == nil {
					
					node = seed // default: first available
				}
			}
		}

		// NOT FOUND ! PEEK: FIRST
		if !ok && node != nil {
			log.Warn(fmt.Sprintf(
				"Preffered service host %q not found; peek=%s addr=%s",
				addr, node.Id, node.Address,
			))
		}
		
		// logger.Info().Msg("SELECT NODE")
		return func() (*registry.Node, error) {
	
			if node == nil {
				return nil, selector.ErrNoneAvailable
			}
	
			return node, nil
		}
	}
}

// PrefferedNode tries to return node
// that exact matches given id
// or selector.Random otherwise
func PrefferedNode(id string) selector.Strategy {
	
	if id == "" {
		return selector.Random
	}

	return func(services []*registry.Service) selector.Next {

		var node *registry.Node
		
		lookup:
		for _, service := range services {
			for _, seed := range service.Nodes {
				if strings.HasSuffix(seed.Id, id) {
					node = seed
					break lookup
				}
			}
		}

		if node == nil {
			log.Warnf(fmt.Sprintf(
				"Preffered service node %q not found; peek random", id,
			))
			return selector.Random(services)
		}
		
		// logger.Info().Msg("SELECT NODE")
		return func() (*registry.Node, error) {

			return node, nil
		}
	}
}
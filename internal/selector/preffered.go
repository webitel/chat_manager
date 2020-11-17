package util

import (
	"strings"

	"github.com/micro/go-micro/v2/registry"
	"github.com/micro/go-micro/v2/client/selector"
)

// PrefferedHost returns node
// that matches given addr prefix
// or any available otherwise
func PrefferedHost(addr string) selector.Strategy {
	return func(services []*registry.Service) selector.Next {
				
		var node *registry.Node
		
		lookup:
		for _, service := range services {
			for _, seed := range service.Nodes {
				if node == nil {
					node = seed // default: first available
				} else if strings.HasPrefix(seed.Address, addr) {
					node = seed
					break lookup
				}
			}
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


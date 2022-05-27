package cmd

import (
	"strings"

	log "github.com/micro/micro/v3/service/logger"
	"github.com/micro/micro/v3/util/selector"
	"github.com/micro/micro/v3/util/selector/roundrobin"
)

type prefSelector struct {
	selector.Selector
	prefHost string
}

func NewSelector(preffered string, defaults selector.Selector) selector.Selector {
	if preffered == "" {
		preffered = "127.0.0.1"
	}

	if defaults == nil {
		defaults = roundrobin.NewSelector()
	}

	return &prefSelector{
		prefHost: preffered,
		Selector: defaults,
	}
}

var _ selector.Selector = (*prefSelector)(nil)

// Select a route from the pool using the strategy
func (c *prefSelector) Select(hosts []string, opts ...selector.SelectOption) (selector.Next, error) {
	var node string
	for _, addr := range hosts {
		if strings.HasPrefix(addr, c.prefHost) {
			node = addr
			break
		}
	}

	if node == "" {
		log.Warnf("preffered host=%s not found; peer=%s", c.prefHost, c.Selector.String())
		return c.Selector.Select(hosts, opts...)
	}

	return func() string {
		log.Infof("preffered peer=%s", node)
		return node
	}, nil
}

// Record the error returned from a route to inform future selection
func (c *prefSelector) Record(_ string, _ error) error {
	return nil
}

// Reset the selector
func (c *prefSelector) Reset() error {
	return nil
}

// String returns the name of the selector
func (c *prefSelector) String() string {
	return "preffered"
}

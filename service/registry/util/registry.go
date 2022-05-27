package util

import "github.com/micro/micro/v3/service/registry"

// Copy makes a copy of service
func CopyService(service *registry.Service) *registry.Service {
	// copy service
	s := new(registry.Service)
	*s = *service

	// copy nodes
	if n := len(service.Nodes); n != 0 {
		page := make([]registry.Node, n)
		list := make([]*registry.Node, n)
		for i, node := range service.Nodes {
			n := &page[i]
			*n = *node
			list[i] = n
		}
		s.Nodes = list
	}

	// copy endpoints
	if n := len(service.Endpoints); n != 0 {
		page := make([]registry.Endpoint, n)
		list := make([]*registry.Endpoint, n)
		for i, ep := range service.Endpoints {
			e := &page[i]
			*e = *ep
			list[i] = e
		}
		s.Endpoints = list
	}

	return s
}

// Copy makes a copy of services
func CopyServices(current []*registry.Service) []*registry.Service {
	services := make([]*registry.Service, len(current))
	for i, service := range current {
		services[i] = CopyService(service)
	}
	return services
}

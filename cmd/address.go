package cmd

import (
	"errors"
	"fmt"
	"net"

	"github.com/micro/micro/v3/service/server"
)

var ErrNoPrivateAddr = errors.New("no private network interface address found")

func GetPrivateAddr() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	var v6 string
	for _, i := range ifaces {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := i.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			default:
				continue
			}
			if !ip.IsPrivate() {
				continue
			}
			if ip.To4() != nil {
				return ip.String(), nil
			}
			if v6 == "" {
				v6 = ip.String()
			}
		}
	}
	if v6 != "" {
		return v6, nil
	}
	return "", ErrNoPrivateAddr
}

func InitServerAddress(srv server.Server) error {
	host, port, err := net.SplitHostPort(srv.Options().Address)
	if err != nil {
		return fmt.Errorf("invalid server address: %w", err)
	}
	if host != "" {
		return nil
	}

	if host, err = GetPrivateAddr(); err != nil {
		return fmt.Errorf("resolve default grpc address: %w; set MICRO_SERVICE_ADDRESS explicitly", err)
	}
	return srv.Init(server.Address(net.JoinHostPort(host, port)))
}

package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"

	"github.com/micro/micro/v3/service/server"
	"github.com/webitel/webitel-go-kit/pkg/hostaddr"
)

func InitServerAddress(srv server.Server) error {
	opts := srv.Options()
	host, port, err := net.SplitHostPort(opts.Address)
	if err != nil {
		return fmt.Errorf("invalid server address: %w", err)
	}

	if host == "" {
		ip, err := hostaddr.PrivateIPv4()
		switch {
		case errors.Is(err, hostaddr.ErrNoPrivateIPv4):
			ip = netip.AddrFrom4([4]byte{127, 0, 0, 1})
			slog.Warn("no private IPv4 address found, grpc listens on loopback; set MICRO_SERVICE_ADDRESS to accept remote connections", slog.String("address", ip.String()))
		case err != nil:
			return fmt.Errorf("resolve default grpc address: %w; set MICRO_SERVICE_ADDRESS explicitly", err)
		}
		return srv.Init(server.Address(net.JoinHostPort(ip.String(), port)))
	}

	listen, err := netip.ParseAddrPort(opts.Address)
	if err != nil || !listen.Addr().IsUnspecified() || opts.Advertise != "" {
		return nil
	}
	if listen.Port() == 0 {
		return fmt.Errorf("server address %q listens on all interfaces with a random port, so the advertise port is unknown; set a fixed port in MICRO_SERVICE_ADDRESS", opts.Address)
	}
	advertise, err := hostaddr.Advertise(listen)
	if err != nil {
		return fmt.Errorf("resolve grpc advertise address: %w; set MICRO_SERVICE_ADDRESS to a specific IP", err)
	}
	return srv.Init(server.Advertise(advertise.String()))
}

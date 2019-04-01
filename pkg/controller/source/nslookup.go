package source

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/xiaopal/kube-service-importer/pkg/fluconf"
)

func intsInclude(ints []int, val int) []int {
	for _, i := range ints {
		if i == val {
			return ints
		}
	}
	return append(ints, val)
}

func stringsInclude(strings []string, str string) []string {
	for _, s := range strings {
		if s == str {
			return strings
		}
	}
	return append(strings, str)
}

func nslookupSourceLoader(conf fluconf.Config) (LoadFunc, string, error) {
	host, srv, port, protocol, overwrite, name := conf.GetString("host", ""), conf.GetString("srv", ""),
		conf.GetInt("port", 0),
		strings.ToUpper(conf.GetString("protocol", "")),
		conf.GetBool("overwrite", false), ""
	var lookup func(context.Context, *log.Logger) ([]string, []int, error)
	if srv != "" {
		parts := strings.SplitN(srv, ".", 3)
		switch parts[1] {
		case "_tcp":
			protocol = "TCP"
		case "_udp":
			protocol = "UDP"
		default:
			if protocol == "" {
				return nil, "", fmt.Errorf("illegal srv %v", srv)
			}
		}
		name, lookup = fmt.Sprintf("nslookup|SRV=%s", srv), func(ctx context.Context, logger *log.Logger) ([]string, []int, error) {
			_, addrs, err := net.DefaultResolver.LookupSRV(ctx, "", "", srv)
			if err != nil {
				return nil, nil, err
			}
			ips, ports := []string{}, []int{}
			for _, addr := range addrs {
				ports = intsInclude(ports, int(addr.Port))
				if ip := net.ParseIP(addr.Target); ip != nil {
					ips = stringsInclude(ips, ip.String())
				} else if ipaddrs, err := net.DefaultResolver.LookupIPAddr(ctx, addr.Target); err == nil {
					for _, ipaddr := range ipaddrs {
						ips = stringsInclude(ips, ipaddr.IP.String())
					}
				} else {
					logger.Printf("lookup host: %v", err)
				}
			}
			if len(ips) > 0 {
				return ips, ports, nil
			}
			return nil, nil, fmt.Errorf("lookup srv failed")
		}
	} else if host != "" {
		if port <= 0 {
			return nil, "", fmt.Errorf("illegal port %v", port)
		}
		if protocol == "" {
			protocol = "TCP"
		}
		name, lookup = fmt.Sprintf("nslookup|%s:%d/%s", host, port, protocol), func(ctx context.Context, logger *log.Logger) ([]string, []int, error) {
			addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, nil, err
			}
			ips := make([]string, len(addrs))
			for i, addr := range addrs {
				ips[i] = addr.IP.String()
			}
			return ips, []int{port}, nil
		}
	} else {
		return nil, "", fmt.Errorf("illegal nslookup %v", conf)
	}
	return func(ctx context.Context, timeout time.Duration, logger *log.Logger) (*LoadResult, error) {
		ips, ports, err := lookup(ctx, logger)
		if err != nil {
			return nil, err
		}
		return &LoadResult{
			IPs:       ips,
			Ports:     ports,
			Protocol:  protocol,
			Overwrite: overwrite,
		}, nil
	}, name, nil
}

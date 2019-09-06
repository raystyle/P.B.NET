package dnsclient

import (
	"net"

	"project/internal/dns"
)

func system_resolve(domain string, Type dns.Type) ([]string, error) {
	address, err := net.LookupHost(domain)
	if err != nil {
		return nil, err
	}
	var (
		ipv4_list []string
		ipv6_list []string
	)
	for _, addr := range address {
		ip := net.ParseIP(addr)
		if ip != nil {
			ipv4 := ip.To4()
			if ipv4 != nil {
				ipv4_list = append(ipv4_list, ipv4.String())
			} else {
				ipv6 := ip.To16()
				if ipv6 != nil {
					ipv6_list = append(ipv6_list, ipv6.String())
				}
			}
		}
	}
	switch Type {
	case dns.IPV4:
		return ipv4_list, nil
	case dns.IPV6:
		return ipv6_list, nil
	default:
		return nil, dns.ErrInvalidType
	}
}

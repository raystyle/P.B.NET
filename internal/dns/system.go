package dns

import (
	"net"
)

func systemResolve(domain string, Type Type) ([]string, error) {
	addrs, err := net.LookupHost(domain)
	if err != nil {
		return nil, err
	}
	var (
		ipv4List []string
		ipv6List []string
	)
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		ipv4 := ip.To4()
		if ipv4 != nil {
			ipv4List = append(ipv4List, ipv4.String())
		} else {
			ipv6List = append(ipv6List, ip.To16().String())
		}
	}
	switch Type {
	case IPv4:
		return ipv4List, nil
	case IPv6:
		return ipv6List, nil
	default:
		return nil, ErrInvalidType
	}
}

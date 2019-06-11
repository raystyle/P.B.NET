package bootstrap

import (
	"errors"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"

	"project/internal/dns"
	"project/internal/global/dnsclient"
)

const test_domain string = "www.baidu.com"

func Test_DNS(t *testing.T) {
	DNS := New_DNS(&mock_resolver{})
	_, _ = DNS.Generate(nil)
	b, err := DNS.Marshal()
	require.Nil(t, err, err)
	err = DNS.Unmarshal(b)
	require.Nil(t, err, err)
	nodes, _ := DNS.Resolve()
	spew.Dump(nodes)

}

type mock_resolver struct {
}

func (this *mock_resolver) Resolve(domain string, opts *dnsclient.Options) ([]string, error) {
	if domain != test_domain {
		return nil, errors.New("domain changed")
	}
	switch opts.Opts.Type {
	case "", dns.IPV4:
		return []string{"127.0.0.1", "192.168.1.11"}, nil
	case dns.IPV6:
		return []string{"[::1]", "[fe80::5456:5f8:1690:5792]"}, nil
	default:
		panic(&dns_panic{Err: dns.ERR_INVALID_TYPE})
	}
}

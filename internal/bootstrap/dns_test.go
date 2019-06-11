package bootstrap

import (
	"testing"

	"project/internal/global/dnsclient"
)

func Test_DNS(t *testing.T) {

}

type mock_resolver struct {
}

func (this *mock_resolver) Resolve(domain string, opts *dnsclient.Options) ([]string, error) {

}

package provider

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	dns "github.com/Edge-Center/edgecenter-dns-sdk-go"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/provider"
)

const (
	ProviderName  = "edgecenter"
	ENV_API_URL   = "EC_API_URL"
	ENV_API_TOKEN = "EC_API_TOKEN"
)

type DnsProvider struct {
	provider.BaseProvider
	client *dns.Client
}

func NewProvider(domainFilter endpoint.DomainFilter, apiUrl, apiToken string) (p *DnsProvider, err error) {
	log.Infof("init %s provider with filters=%+v", ProviderName, domainFilter.Filters)

	if apiToken == "" {
		return nil, errors.New("empty API token, check env var " + ENV_API_TOKEN)
	}

	client := dns.NewClient(dns.PermanentAPIKeyAuth(apiToken))
	if apiUrl != "" {
		client.BaseURL, err = url.Parse(apiUrl)
		if err != nil {
			return nil, fmt.Errorf("can't parse API URL '%s'", apiUrl)
		}
	}

	return &DnsProvider{
		client: client,
	}, nil
}

// todo
func (p *DnsProvider) Records(ctx context.Context) ([]*endpoint.Endpoint, error) {
	return nil, nil
}

// todo
func (p *DnsProvider) ApplyChanges(ctx context.Context, changes *plan.Changes) error {
	return nil
}

// todo
func (p *DnsProvider) GetDomainFilter() *endpoint.DomainFilter {
	return nil
}

// todo
func (p *DnsProvider) AdjustEndpoints(endpoints []*endpoint.Endpoint) ([]*endpoint.Endpoint, error) {
	return nil, nil
}

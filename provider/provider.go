package provider

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	dns "github.com/Edge-Center/edgecenter-dns-sdk-go"
	"github.com/Edge-Center/external-dns-ec-webhook/log"
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
	log.Logger(context.Background()).Infof("init %s provider with filters=%+v", ProviderName, domainFilter.Filters)

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

func (p *DnsProvider) Records(ctx context.Context) ([]*endpoint.Endpoint, error) {
	logger := log.Logger(ctx)
	logger.Info("starting to get records")
	defer logger.Info("finished getting records")

	filters := p.GetDomainFilter(ctx).Filters
	logger = log.Logger(ctx).WithField("filters", filters)

	// todo mb add context with timeout

	zones, err := p.client.ZonesWithRecords(ctx, func(zone *dns.ZonesFilter) {
		zone.Names = filters
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get zones with records: %s")
	}

	recordCountByZone := make(map[string]int)
	result := make([]*endpoint.Endpoint, 0)

	for _, zone := range zones {
		recordCountByZone[zone.Name]++
		for _, r := range zone.Records {
			if provider.SupportedRecordType(r.Type) {
				result = append(result,
					endpoint.NewEndpointWithTTL(r.Name, r.Type, endpoint.TTL(r.TTL), r.ShortAnswers...))
			}
		}
	}

	logger.
		WithField("recordCountByZone", recordCountByZone).
		WithField("result", result).
		Debugf("found %d zones, %d records in result", len(recordCountByZone), len(result))

	return result, nil
}

// todo
func (p *DnsProvider) ApplyChanges(ctx context.Context, changes *plan.Changes) error {
	return nil
}

func (p *DnsProvider) GetDomainFilter(ctx context.Context) *endpoint.DomainFilter {
	logger := log.Logger(ctx)
	logger.Info("start GetDomainFilter")
	defer logger.Info("finish GetDomainFilter")

	zones, err := p.client.ZonesWithRecords(ctx, nil)
	if err != nil {
		logger.Errorf("failed to get zones with records: %s", err)
		return &endpoint.DomainFilter{}
	}

	domains := make([]string, 0)
	for _, z := range zones {
		domains = append(domains, z.Name, z.Name+".")
	}
	return endpoint.NewDomainFilter(domains)
}

// todo
func (p *DnsProvider) AdjustEndpoints(endpoints []*endpoint.Endpoint) ([]*endpoint.Endpoint, error) {
	return nil, nil
}

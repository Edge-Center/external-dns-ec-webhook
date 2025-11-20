package provider

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	dns "github.com/Edge-Center/edgecenter-dns-sdk-go"
	"github.com/Edge-Center/external-dns-ec-webhook/log"
	"golang.org/x/sync/errgroup"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/provider"
)

const (
	ProviderName  = "edgecenter"
	ENV_API_URL   = "EC_API_URL"
	ENV_API_TOKEN = "EC_API_TOKEN"
	ENV_DRY_RUN   = "EC_DRY_RUN"
)

// DnsClient is an interface for test purposes
type DnsClient interface {
	AddZoneRRSet(ctx context.Context,
		zone, recordName, recordType string,
		values []dns.ResourceRecord, ttl int, opts ...dns.AddZoneOpt) error
	ZonesWithRecords(ctx context.Context, filters ...func(zone *dns.ZonesFilter)) ([]dns.Zone, error)
	DeleteRRSetRecord(ctx context.Context, zone, name, recordType string, contents ...string) error
}

type DnsProvider struct {
	provider.BaseProvider
	client DnsClient
	dryRun bool
}

func NewProvider(apiUrl, apiToken string, dryRun bool) (p *DnsProvider, err error) {
	log.Logger(context.Background()).Infof("init %s provider for %s", ProviderName, apiUrl)

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
		dryRun: dryRun,
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
		return nil, fmt.Errorf("failed to get zones with records: %s", err)
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

const changesApplicationStepsNumber = 3

// todo mb add context with timeout
func (p *DnsProvider) ApplyChanges(ctx context.Context, changes *plan.Changes) error {
	if !changes.HasChanges() {
		return nil
	}

	logger := log.Logger(ctx)
	logger.Info("starting to apply changes")
	defer logger.Info("finished applying changes")

	getZoneFunc := p.zoneFromDNSNameGetter(ctx)
	appliedChanges := struct {
		created int
		updated int
		deleted int
	}{}

	var updateGr *errgroup.Group
	appliedChanges.updated, updateGr = p.handleUpdateChanges(ctx, changes, getZoneFunc)

	var deleteGr *errgroup.Group
	appliedChanges.deleted, deleteGr = p.handleDeleteChanges(ctx, changes, getZoneFunc)

	var createGr *errgroup.Group
	appliedChanges.created, createGr = p.handleCreateChanges(ctx, changes, getZoneFunc)

	logger = logger.WithField("to_apply", appliedChanges)

	errs := make([]error, 0, changesApplicationStepsNumber)
	err := updateGr.Wait()
	if err != nil {
		logger.WithField(log.ErrorKey, err).Error("failed to commit update changes")
		errs = append(errs, err)
	} else {
		logger.Info("update changes commited")
	}
	err = deleteGr.Wait()
	if err != nil {
		logger.WithField(log.ErrorKey, err).Error("failed to commit delete changes")
		errs = append(errs, err)
	} else {
		logger.Info("delete changes commited")
	}
	err = createGr.Wait()
	if err != nil {
		logger.WithField(log.ErrorKey, err).Error("failed to commit create changes")
		errs = append(errs, err)
	} else {
		logger.Info("create changes commited")
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (p *DnsProvider) GetDomainFilter(ctx context.Context) *endpoint.DomainFilter {
	logger := log.Logger(ctx)
	logger.Info("start GetDomainFilter")
	defer logger.Info("finish GetDomainFilter")

	zones, err := p.client.ZonesWithRecords(ctx)
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

// todo figure out wtf is this func
func (p *DnsProvider) AdjustEndpoints(endpoints []*endpoint.Endpoint) ([]*endpoint.Endpoint, error) {
	adjusted := make([]*endpoint.Endpoint, 0, len(endpoints))
	for _, e := range endpoints {
		if e.RecordType != "TXT" {
			adjusted = append(adjusted, e)
		}
	}
	return adjusted, nil
}

func (p *DnsProvider) zoneFromDNSNameGetter(ctx context.Context) func(name string) (zone string) {
	existingZones := p.GetDomainFilter(ctx)
	search := make(map[string]string)
	for _, zone := range existingZones.Filters {
		search[zone] = strings.Trim(zone, ".")
	}
	return func(name string) (zone string) {
		dnsName := strings.Trim(name, ".")
		if result, ok := search[dnsName]; ok {
			return result
		}
		i, j := 0, 0
		for j != -1 { // check if there was a dot
			if result, ok := search[dnsName[i:]]; ok { // check substring to the right of it
				return result
			}
			j = strings.Index(dnsName[i:], ".") // look for the next dot
			i = i + j + 1                       // calculate index of the next substr beginning
		}
		return ""
	}
}

func (p *DnsProvider) handleUpdateChanges(ctx context.Context, changes *plan.Changes, getZone func(name string) string) (int, *errgroup.Group) {
	logger := log.Logger(ctx)
	logger.Info("start applying Update changes")
	defer logger.Info("finish applying Update changes")

	var forUpdate int

	gr, _ := errgroup.WithContext(ctx)

	for _, e := range changes.UpdateNew {
		zone := getZone(e.DNSName)
		if zone == "" {
			logger.WithField(log.DNSNameKey, e.DNSName).Warning("update skipped - no such zone")
			continue
		}

		rrsetsToDelete := findRecordsToDelete(e, changes.UpdateOld)
		for _, content := range rrsetsToDelete {
			msg := fmt.Sprintf("for update-delete %s %s %s", e.DNSName, e.RecordType, content)
			if p.dryRun {
				logger.WithField(log.DryRunKey, true).Info(msg)
				continue
			}
			logger.Debug(msg)
		}

		rrsetsToCreate := findRecordsToCreate(e, changes.UpdateOld)
		recordValues := make([]dns.ResourceRecord, 0, len(rrsetsToCreate))

		for _, content := range rrsetsToCreate {
			msg := fmt.Sprintf("for update-add %s %s %s", e.DNSName, e.RecordType, content)
			if p.dryRun {
				logger.WithField(log.DryRunKey, true).Info(msg)
				continue
			}
			logger.Debug(msg)
			rr := dns.ResourceRecord{Enabled: true}
			rr.SetContent(e.RecordType, content)
			recordValues = append(recordValues, rr)
		}

		forUpdate += len(rrsetsToDelete) + len(rrsetsToCreate)

		gr.Go(func() error {
			if len(rrsetsToDelete) > 0 {
				err := p.client.DeleteRRSetRecord(ctx, zone, e.DNSName, e.RecordType, rrsetsToDelete...)
				if err != nil {
					err = fmt.Errorf("failed to delete rrset records: %s", err)
				}
				logger.Error(err)
				return err
			}
			if len(rrsetsToCreate) > 0 {
				err := p.client.AddZoneRRSet(ctx, zone, e.DNSName, e.RecordType, recordValues, int(e.RecordTTL))
				if err != nil {
					err = fmt.Errorf("failed to add rrset records: %s", err)
				}
				logger.Error(err)
				return err
			}
			return nil
		})
	}

	return forUpdate, gr
}

func (p *DnsProvider) handleDeleteChanges(ctx context.Context, changes *plan.Changes, getZone func(name string) string) (int, *errgroup.Group) {
	logger := log.Logger(ctx)
	logger.Info("start applying Delete changes")
	defer logger.Info("finish applying Delete changes")

	var forDelete int
	gr, _ := errgroup.WithContext(ctx)

	for _, e := range changes.Delete {
		zone := getZone(e.DNSName)
		if zone == "" {
			logger.WithField(log.DNSNameKey, e.DNSName).Warning("delete skipped - no such zone")
			continue
		}

		forDelete += len(e.Targets)

		for _, content := range e.Targets {
			msg := fmt.Sprintf("for delete %s %s %s", e.DNSName, e.RecordType, content)
			if p.dryRun {
				logger.WithField(log.DryRunKey, true).Info(msg)
				continue
			}
			logger.Debug(msg)
		}

		if len(e.Targets) > 0 {
			gr.Go(func() error {
				err := p.client.DeleteRRSetRecord(ctx, zone, e.DNSName, e.RecordType, e.Targets...)
				if err != nil {
					err = fmt.Errorf("failed to delete rrset: %s", err)
				}
				logger.Error(err)
				return err
			})
		}
	}

	return forDelete, gr
}

func (p *DnsProvider) handleCreateChanges(ctx context.Context, changes *plan.Changes, getZone func(name string) string) (int, *errgroup.Group) {
	logger := log.Logger(ctx)
	logger.Info("start applying Create changes")
	defer logger.Info("finish applying Create changes")

	var forCreate int
	gr, _ := errgroup.WithContext(ctx)

	for _, e := range changes.Create {
		zone := getZone(e.DNSName)
		if zone == "" {
			logger.WithField(log.DNSNameKey, e.DNSName).Warning("delete skipped - no such zone")
			continue
		}

		forCreate += len(e.Targets)

		recordValues := make([]dns.ResourceRecord, 0)
		for _, content := range e.Targets {
			msg := fmt.Sprintf("for create %s %s %s", e.DNSName, e.RecordType, content)
			if p.dryRun {
				logger.WithField(log.DryRunKey, true).Info(msg)
				continue
			}
			logger.Debug(msg)
			rr := dns.ResourceRecord{Enabled: true}
			rr.SetContent(e.RecordType, content)
			recordValues = append(recordValues, rr)
		}

		if len(e.Targets) > 0 {
			gr.Go(func() error {
				err := p.client.AddZoneRRSet(ctx, zone, e.DNSName, e.RecordType, recordValues, int(e.RecordTTL))
				if err != nil {
					err = fmt.Errorf("failed to create rrset: %s", err)
				}
				logger.Error(err)
				return err
			})
		}
	}

	return forCreate, gr
}

func findRecordsToDelete(update *endpoint.Endpoint, existingEndpoints []*endpoint.Endpoint) endpoint.Targets {
	var existing *endpoint.Endpoint
	for _, ex := range existingEndpoints {
		if ex.RecordType != update.RecordType || ex.DNSName != update.DNSName {
			continue
		}
		existing = ex
	}
	if existing == nil {
		return nil
	}
	return findDiff(existing, update)
}

func findRecordsToCreate(update *endpoint.Endpoint, existingEndpoints []*endpoint.Endpoint) endpoint.Targets {
	var existing *endpoint.Endpoint
	for _, ex := range existingEndpoints {
		if ex.RecordType != update.RecordType || ex.DNSName != update.DNSName {
			continue
		}
		existing = ex
	}
	if existing == nil {
		return nil
	}
	return findDiff(update, existing)
}

// findDiff returns RRSets in target that don't exist in source
func findDiff(target, source *endpoint.Endpoint) endpoint.Targets {
	res := endpoint.Targets{}
	for _, t := range target.Targets {
		exists := false
		for _, st := range source.Targets {
			if st == t {
				exists = true
				break
			}
		}
		if !exists {
			res = append(res, t)
		}
	}
	return res
}

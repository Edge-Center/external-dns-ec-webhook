package provider

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	dns "github.com/Edge-Center/edgecenter-dns-sdk-go"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
)

type clientMock struct {
	addZoneRRSet      func(ctx context.Context, zone, recordName, recordType string, values []dns.ResourceRecord, ttl int, opts ...dns.AddZoneOpt) error
	zonesWithRecords  func(ctx context.Context, filters ...func(zone *dns.ZonesFilter)) ([]dns.Zone, error)
	deleteRRSetRecord func(ctx context.Context, zone, name, recordType string, contents ...string) error
}

func (c *clientMock) AddZoneRRSet(ctx context.Context,
	zone, recordName, recordType string,
	values []dns.ResourceRecord, ttl int, opts ...dns.AddZoneOpt) error {
	return c.addZoneRRSet(ctx, zone, recordName, recordType, values, ttl, opts...)
}

func (c *clientMock) ZonesWithRecords(ctx context.Context, filters ...func(zone *dns.ZonesFilter)) ([]dns.Zone, error) {
	return c.zonesWithRecords(ctx, filters...)
}

func (c *clientMock) DeleteRRSetRecord(ctx context.Context, zone, name, recordType string, contents ...string) error {
	return c.deleteRRSetRecord(ctx, zone, name, recordType, contents...)
}

func Test_dnsProvider_Records(t *testing.T) {
	type fields struct {
		domainFilter endpoint.DomainFilter
		client       DnsClient
		dryRun       bool
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []endpoint.Endpoint
		wantErr bool
	}{
		{
			name: "no_filter",
			fields: fields{
				domainFilter: endpoint.DomainFilter{},
				client: &clientMock{
					zonesWithRecords: func(ctx context.Context,
						filters ...func(zone *dns.ZonesFilter)) ([]dns.Zone, error) {
						return []dns.Zone{
							{
								Name: "example.com",
								Records: []dns.ZoneRecord{
									{
										Name:         "test.example.com",
										Type:         "A",
										TTL:          10,
										ShortAnswers: []string{"1.1.1.1"},
									},
								},
							},
						}, nil
					},
				},
				dryRun: false,
			},
			args: args{
				ctx: context.Background(),
			},
			want: []endpoint.Endpoint{
				*endpoint.NewEndpointWithTTL(
					"test.example.com", "A", endpoint.TTL(10), []string{"1.1.1.1"}...),
			},
			wantErr: false,
		},
		{
			name: "filtered",
			fields: fields{
				domainFilter: endpoint.DomainFilter{Filters: []string{"example.com"}},
				client: &clientMock{
					zonesWithRecords: func(ctx context.Context,
						filters ...func(zone *dns.ZonesFilter)) ([]dns.Zone, error) {
						return []dns.Zone{
							{
								Name: "example.com",
								Records: []dns.ZoneRecord{
									{
										Name:         "test.example.com",
										Type:         "A",
										TTL:          10,
										ShortAnswers: []string{"1.1.1.1"},
									},
								},
							},
						}, nil
					},
				},
				dryRun: false,
			},
			args: args{
				ctx: context.Background(),
			},
			want: []endpoint.Endpoint{
				*endpoint.NewEndpointWithTTL(
					"test.example.com", "A", endpoint.TTL(10), []string{"1.1.1.1"}...),
			},
			wantErr: false,
		},
		{
			name: "error",
			fields: fields{
				domainFilter: endpoint.DomainFilter{Filters: []string{"example.com"}},
				client: &clientMock{
					zonesWithRecords: func(ctx context.Context,
						filters ...func(zone *dns.ZonesFilter)) ([]dns.Zone, error) {
						return nil, fmt.Errorf("test")
					},
				},
				dryRun: false,
			},
			args: args{
				ctx: context.Background(),
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &DnsProvider{
				client: tt.fields.client,
				dryRun: tt.fields.dryRun,
			}
			got, err := p.Records(tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Records() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			toCompare := make([]endpoint.Endpoint, len(got))
			for i, e := range got {
				toCompare[i] = *e
			}
			if !reflect.DeepEqual(toCompare, tt.want) {
				t.Errorf("Records() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_dnsProvider_GetDomainFilter(t *testing.T) {
	type fields struct {
		domainFilter endpoint.DomainFilter
		client       DnsClient
		dryRun       bool
	}
	tests := []struct {
		name   string
		fields fields
		want   *endpoint.DomainFilter
	}{
		{
			name: "empty",
			fields: fields{
				domainFilter: endpoint.DomainFilter{},
				client: &clientMock{
					zonesWithRecords: func(ctx context.Context,
						filters ...func(zone *dns.ZonesFilter)) ([]dns.Zone, error) {
						return []dns.Zone{}, nil
					},
				},
				dryRun: false,
			},
			want: endpoint.NewDomainFilter([]string{}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &DnsProvider{
				client: tt.fields.client,
				dryRun: tt.fields.dryRun,
			}
			if got := p.GetDomainFilter(context.Background()); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetDomainFilter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_dnsProvider_ApplyChanges(t *testing.T) {
	type fields struct {
		domainFilter endpoint.DomainFilter
		client       DnsClient
		dryRun       bool
	}
	type args struct {
		ctx     context.Context
		changes *plan.Changes
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "delete exist in filter",
			fields: fields{
				domainFilter: endpoint.DomainFilter{},
				client: &clientMock{
					zonesWithRecords: func(ctx context.Context,
						filters ...func(zone *dns.ZonesFilter)) ([]dns.Zone, error) {
						return []dns.Zone{{Name: "test.com"}}, nil
					},
					deleteRRSetRecord: func(ctx context.Context, zone, name, recordType string, contents ...string) error {
						if zone == "test.com" && name == "my.test.com" && recordType == "A" && contents[0] == "1.1.1.1" {
							return nil
						}
						return fmt.Errorf("deleteRRSetRecord wrong params")
					},
				},
				dryRun: false,
			},
			args: args{
				ctx: context.Background(),
				changes: &plan.Changes{
					Delete: []*endpoint.Endpoint{
						endpoint.NewEndpointWithTTL("my.test.com", "A", 10, "1.1.1.1"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "delete exist in filter with left dot",
			fields: fields{
				domainFilter: endpoint.DomainFilter{},
				client: &clientMock{
					zonesWithRecords: func(ctx context.Context,
						filters ...func(zone *dns.ZonesFilter)) ([]dns.Zone, error) {
						return []dns.Zone{{Name: "test.com"}}, nil
					},
					deleteRRSetRecord: func(ctx context.Context, zone, name, recordType string, contents ...string) error {
						if zone == "test.com" && name == ".my.test.com" && recordType == "A" && contents[0] == "1.1.1.1" {
							return nil
						}
						return fmt.Errorf("deleteRRSetRecord wrong params")
					},
				},
				dryRun: false,
			},
			args: args{
				ctx: context.Background(),
				changes: &plan.Changes{
					Delete: []*endpoint.Endpoint{
						endpoint.NewEndpointWithTTL(".my.test.com", "A", 10, "1.1.1.1"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "delete not exist in filter",
			fields: fields{
				domainFilter: endpoint.DomainFilter{},
				client: &clientMock{
					zonesWithRecords: func(ctx context.Context,
						filters ...func(zone *dns.ZonesFilter)) ([]dns.Zone, error) {
						return []dns.Zone{{Name: "test.com"}}, nil
					},
				},
				dryRun: false,
			},
			args: args{
				ctx: context.Background(),
				changes: &plan.Changes{
					Delete: []*endpoint.Endpoint{
						endpoint.NewEndpointWithTTL("my.t.com", "A", 10, "1.1.1.1"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "delete error",
			fields: fields{
				domainFilter: endpoint.DomainFilter{},
				client: &clientMock{
					zonesWithRecords: func(ctx context.Context,
						filters ...func(zone *dns.ZonesFilter)) ([]dns.Zone, error) {
						return []dns.Zone{{Name: "test.com"}}, nil
					},
					deleteRRSetRecord: func(ctx context.Context, zone, name, recordType string, contents ...string) error {
						if zone == "test.com" && name == "my.test.com" && recordType == "A" && contents[0] == "1.1.1.1" {
							return nil
						}
						return fmt.Errorf("deleteRRSetRecord wrong params")
					},
				},
				dryRun: false,
			},
			args: args{
				ctx: context.Background(),
				changes: &plan.Changes{
					Delete: []*endpoint.Endpoint{
						endpoint.NewEndpointWithTTL("my1.test.com", "A", 10, "1.1.1.1"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "create ok",
			fields: fields{
				domainFilter: endpoint.DomainFilter{},
				client: &clientMock{
					zonesWithRecords: func(ctx context.Context,
						filters ...func(zone *dns.ZonesFilter)) ([]dns.Zone, error) {
						return []dns.Zone{{Name: "test.com"}}, nil
					},
					addZoneRRSet: func(ctx context.Context, zone, recordName, recordType string, values []dns.ResourceRecord, ttl int, opts ...dns.AddZoneOpt) error {
						if zone == "test.com" &&
							ttl == 10 &&
							recordName == "my.test.com" &&
							recordType == "A" &&
							values[0].Content[0] == "1.1.1.1" {
							return nil
						}
						return fmt.Errorf("addZoneRRSet wrong params")
					},
				},
				dryRun: false,
			},
			args: args{
				ctx: context.Background(),
				changes: &plan.Changes{
					Create: []*endpoint.Endpoint{
						endpoint.NewEndpointWithTTL("my.test.com", "A", 10, "1.1.1.1"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "update ok",
			fields: fields{
				domainFilter: endpoint.DomainFilter{},
				client: &clientMock{
					zonesWithRecords: func(ctx context.Context,
						filters ...func(zone *dns.ZonesFilter)) ([]dns.Zone, error) {
						return []dns.Zone{{Name: "test.com"}}, nil
					},
					deleteRRSetRecord: func(ctx context.Context, zone, name, recordType string, contents ...string) error {
						if zone == "test.com" && name == "my.test.com" && recordType == "A" && contents[0] == "1.1.1.1" {
							return nil
						}
						return fmt.Errorf("deleteRRSetRecord wrong params: %s %s %s %+v",
							zone, name, recordType, contents)
					},
					addZoneRRSet: func(ctx context.Context, zone, recordName, recordType string, values []dns.ResourceRecord, ttl int, opts ...dns.AddZoneOpt) error {
						if zone == "test.com" &&
							ttl == 10 &&
							recordName == "my.test.com" &&
							recordType == "A" &&
							values[0].Content[0] == "1.2.3.4" {
							return nil
						}
						return fmt.Errorf("addZoneRRSet wrong params")
					},
				},
				dryRun: false,
			},
			args: args{
				ctx: context.Background(),
				changes: &plan.Changes{
					UpdateOld: []*endpoint.Endpoint{
						endpoint.NewEndpointWithTTL("my.test.com", "A", 10, "1.1.1.1"),
						endpoint.NewEndpointWithTTL("my1.test.com", "A", 10, "1.1.1.2"),
					},
					UpdateNew: []*endpoint.Endpoint{
						endpoint.NewEndpointWithTTL("my.test.com", "A", 10, "1.2.3.4"),
						endpoint.NewEndpointWithTTL("my1.test.com", "A", 10, "1.1.1.2"),
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &DnsProvider{
				client: tt.fields.client,
				dryRun: tt.fields.dryRun,
			}
			if err := p.ApplyChanges(tt.args.ctx, tt.args.changes); (err != nil) != tt.wantErr {
				t.Errorf("ApplyChanges() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

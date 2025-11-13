package provider_test

import (
	"context"

	dns "github.com/Edge-Center/edgecenter-dns-sdk-go"
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

// Package ipgeo resolves display-only locations using bundled offline IP databases.
package ipgeo

import (
	"net/netip"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lionsoul2014/ip2region/binding/golang/service"
)

// Resolver provides a location label for a public IP address.
type Resolver interface {
	Lookup(ip string) string
}

// Locator owns the concurrency-safe IPv4 and IPv6 database readers.
type Locator struct {
	reader *service.Ip2Region
}

// New validates and opens both databases with a shared vector index cache.
func New(dataDir string) (*Locator, error) {
	v4, err := service.NewV4Config(service.VIndexCache, filepath.Join(dataDir, "ip2region_v4.xdb"), 4)
	if err != nil {
		return nil, err
	}
	v6, err := service.NewV6Config(service.VIndexCache, filepath.Join(dataDir, "ip2region_v6.xdb"), 4)
	if err != nil {
		return nil, err
	}
	reader, err := service.NewIp2Region(v4, v6)
	if err != nil {
		return nil, err
	}
	return &Locator{reader: reader}, nil
}

func formatLocation(region string) string {
	fields := strings.Split(region, "|")
	parts := make([]string, 0, 3)
	for _, field := range fields[:min(3, len(fields))] {
		field = strings.TrimSpace(field)
		if field != "" && field != "0" && !slices.Contains(parts, field) {
			parts = append(parts, field)
		}
	}
	return strings.Join(parts, " · ")
}

// Lookup returns an empty label when an address has no usable public location.
func (l *Locator) Lookup(ip string) string {
	address, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return ""
	}
	address = address.Unmap()
	if !address.IsGlobalUnicast() || address.IsPrivate() {
		return ""
	}
	region, err := l.reader.Search(address.String())
	if err != nil {
		return ""
	}
	return formatLocation(region)
}

// Close releases the readers after all request handlers have finished.
func (l *Locator) Close() {
	l.reader.Close()
}

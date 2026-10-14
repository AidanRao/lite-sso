package ipgeo

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_FormatLocation_FormatsGeographicFields(t *testing.T) {
	for _, test := range []struct {
		name   string
		region string
		want   string
	}{
		{name: "municipality", region: "中国|北京|北京|联通|CN", want: "中国 · 北京"},
		{name: "province and city", region: "中国|广东省|深圳市|电信|CN", want: "中国 · 广东省 · 深圳市"},
		{name: "missing city", region: "中国|0|0|0|CN", want: "中国"},
		{name: "empty", region: "0| |0|ISP|ZZ", want: ""},
		{name: "no match", region: "", want: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, formatLocation(test.region))
		})
	}
}

func Test_Lookup_PublicAddressesAndConcurrentQueries(t *testing.T) {
	locator, err := New(filepath.Join("..", "..", "data", "ip2region"))
	require.NoError(t, err)
	t.Cleanup(locator.Close)
	addresses := []string{"113.92.157.29", "240e:3b7:3272:d8d0:db09:c067:8d59:539e"}
	expected := make([]string, len(addresses))
	for index, ip := range addresses {
		expected[index] = locator.Lookup(ip)
		require.Contains(t, expected[index], "中国")
		require.NotContains(t, expected[index], "|")
	}
	var queries sync.WaitGroup
	for range 20 {
		queries.Go(func() {
			for index, ip := range addresses {
				for range 10 {
					assert.Equal(t, expected[index], locator.Lookup(ip))
				}
			}
		})
	}
	queries.Wait()
}

func Test_Lookup_UnavailableLocationDoesNotFail(t *testing.T) {
	locator, err := New(filepath.Join("..", "..", "data", "ip2region"))
	require.NoError(t, err)
	for _, ip := range []string{"", "invalid", "127.0.0.1", "10.0.0.1", "192.168.1.2", "::1", "fc00::1", "::ffff:192.168.1.2"} {
		require.Empty(t, locator.Lookup(ip), ip)
	}
	locator.Close()
	require.Empty(t, locator.Lookup("113.92.157.29"), "query errors must become an empty display value")
}

func Test_New_MissingOrInvalidDatabaseFails(t *testing.T) {
	directory := t.TempDir()
	_, err := New(directory)
	require.Error(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(directory, "ip2region_v4.xdb"), []byte("invalid database"), 0600))
	_, err = New(directory)
	require.Error(t, err)
}

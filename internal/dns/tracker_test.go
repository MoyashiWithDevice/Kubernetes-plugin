package dns

import (
	"testing"

	"Kubernetes-plugin/internal/flow"
	"Kubernetes-plugin/internal/resolver"
)

type fakeDNSReader struct {
	stats []struct {
		key flow.DNSStatsKey
		val flow.DNSStatsVal
	}
}

func (f *fakeDNSReader) ReadDNSStats(fn func(flow.DNSStatsKey, flow.DNSStatsVal) error) error {
	for _, s := range f.stats {
		if err := fn(s.key, s.val); err != nil {
			return err
		}
	}
	return nil
}

func TestTrackerRead(t *testing.T) {
	r := resolver.NewPod(nil, false)
	defer r.Close()

	reader := &fakeDNSReader{
		stats: []struct {
			key flow.DNSStatsKey
			val flow.DNSStatsVal
		}{
			{
				key: flow.DNSStatsKey{
					SrcIP: [4]byte{10, 0, 1, 10},
					DstIP: [4]byte{10, 96, 0, 10},
				},
				val: flow.DNSStatsVal{
					Count: 100,
					SumUs: 5000,
					MinUs: 10,
					MaxUs: 200,
					Hist: [flow.RTTHistBuckets]uint32{
						10, 20, 30, 15, 10, 5, 5, 3, 2, 0,
						0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
						0, 0, 0, 0, 0, 0, 0,
					},
				},
			},
			{
				key: flow.DNSStatsKey{
					SrcIP: [4]byte{10, 0, 1, 20},
					DstIP: [4]byte{10, 96, 0, 10},
				},
				val: flow.DNSStatsVal{
					Count: 50,
					SumUs: 10000,
					MinUs: 50,
					MaxUs: 500,
					Hist: [flow.RTTHistBuckets]uint32{
						0, 0, 0, 0, 5, 10, 15, 10, 5, 3,
						2, 0, 0, 0, 0, 0, 0, 0, 0, 0,
						0, 0, 0, 0, 0, 0, 0,
					},
				},
			},
		},
	}

	tracker := New()
	if err := tracker.Read(reader, r); err != nil {
		t.Fatalf("Read: %v", err)
	}

	entries := tracker.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Sorted by P95 descending; the 2nd pair (p95 ~800µs) should rank first.
	if entries[0].Queries != 50 {
		t.Errorf("expected 50 queries for first entry, got %d", entries[0].Queries)
	}
	if entries[1].Queries != 100 {
		t.Errorf("expected 100 queries for second entry, got %d", entries[1].Queries)
	}
}

func TestFormatDNS(t *testing.T) {
	out := FormatDNS(nil, 10e9)
	if out != "(no DNS data)" {
		t.Errorf("expected empty message, got %q", out)
	}
}

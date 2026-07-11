package latency

import (
	"net"
	"strings"
	"testing"
	"time"

	"Kubernetes-plugin/internal/flow"
)

type stubResolver struct{}

func (s stubResolver) Resolve(ip net.IP) string {
	switch ip.String() {
	case "10.0.1.1":
		return "frontend"
	case "10.0.1.2":
		return "api"
	case "10.0.1.3":
		return "redis"
	}
	return ip.String()
}

func (s stubResolver) Close() {}

type stubCollector struct {
	data []struct {
		key flow.RTTKey
		val flow.RTTVal
	}
}

func (s *stubCollector) ReadRTT(fn func(flow.RTTKey, flow.RTTVal) error) error {
	for i := range s.data {
		if err := fn(s.data[i].key, s.data[i].val); err != nil {
			return err
		}
	}
	return nil
}

func toBytes(ip string) [4]byte {
	parsed := net.ParseIP(ip).To4()
	var b [4]byte
	copy(b[:], parsed)
	return b
}

func TestPercentileFromSamples(t *testing.T) {
	// 1..100
	samples := make([]uint64, 100)
	for i := range samples {
		samples[i] = uint64(i + 1)
	}
	p95 := PercentileFromSamples(samples, 0.95)
	p99 := PercentileFromSamples(samples, 0.99)
	if p95 != 95 {
		t.Fatalf("p95: got %v want 95", p95)
	}
	if p99 != 99 {
		t.Fatalf("p99: got %v want 99", p99)
	}
}

func TestPercentileFromSamplesEmpty(t *testing.T) {
	if got := PercentileFromSamples(nil, 0.95); got != 0 {
		t.Fatalf("expected 0, got %v", got)
	}
}

func TestPercentileFromHist(t *testing.T) {
	// Put 100 samples in bucket 10 (covers [1024, 2048) → midpoint 1536)
	hist := make([]uint32, flow.RTTHistBuckets)
	hist[10] = 95
	hist[12] = 4
	hist[15] = 1
	// count = 100; p95 lands in bucket 10; p99 in bucket 12
	p95 := PercentileFromHist(hist, 100, 0.95)
	p99 := PercentileFromHist(hist, 100, 0.99)

	// bucket 10 midpoint = (1024+2048)/2 = 1536
	if p95 != 1536 {
		t.Fatalf("p95: got %v want 1536", p95)
	}
	// bucket 12 midpoint = (4096+8192)/2 = 6144
	if p99 != 6144 {
		t.Fatalf("p99: got %v want 6144", p99)
	}
}

func TestPercentileFromHistEmpty(t *testing.T) {
	if got := PercentileFromHist(make([]uint32, flow.RTTHistBuckets), 0, 0.95); got != 0 {
		t.Fatalf("expected 0, got %v", got)
	}
}

func TestTrackerReadAndAggregate(t *testing.T) {
	// Two connections frontend→api, one api→redis with higher latency
	var histLow, histHigh [flow.RTTHistBuckets]uint32
	histLow[8] = 10  // ~256–512µs
	histHigh[14] = 5 // ~16–32ms

	c := &stubCollector{
		data: []struct {
			key flow.RTTKey
			val flow.RTTVal
		}{
			{
				key: flow.RTTKey{SrcIP: toBytes("10.0.1.1"), DstIP: toBytes("10.0.1.2"), SrcPort: 1, DstPort: 80},
				val: flow.RTTVal{SumUs: 3000, Count: 10, MinUs: 200, MaxUs: 500, Hist: histLow},
			},
			{
				key: flow.RTTKey{SrcIP: toBytes("10.0.1.1"), DstIP: toBytes("10.0.1.2"), SrcPort: 2, DstPort: 80},
				val: flow.RTTVal{SumUs: 4000, Count: 10, MinUs: 250, MaxUs: 600, Hist: histLow},
			},
			{
				key: flow.RTTKey{SrcIP: toBytes("10.0.1.2"), DstIP: toBytes("10.0.1.3"), SrcPort: 3, DstPort: 6379},
				val: flow.RTTVal{SumUs: 100000, Count: 5, MinUs: 10000, MaxUs: 30000, Hist: histHigh},
			},
		},
	}

	tracker := New()
	var r stubResolver
	if err := tracker.Read(c, r); err != nil {
		t.Fatalf("Read: %v", err)
	}

	entries := tracker.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Sorted by p95 desc → api→redis first
	if entries[0].Source != "api" || entries[0].Destination != "redis" {
		t.Fatalf("expected api→redis first, got %s→%s", entries[0].Source, entries[0].Destination)
	}
	if entries[0].Samples != 5 {
		t.Fatalf("expected 5 samples, got %d", entries[0].Samples)
	}

	// frontend→api aggregated: count 20, sum 7000, avg 350
	if entries[1].Source != "frontend" || entries[1].Destination != "api" {
		t.Fatalf("expected frontend→api second, got %s→%s", entries[1].Source, entries[1].Destination)
	}
	if entries[1].Samples != 20 {
		t.Fatalf("expected 20 samples, got %d", entries[1].Samples)
	}
	if entries[1].AvgUs != 350 {
		t.Fatalf("expected avg 350µs, got %v", entries[1].AvgUs)
	}
	if entries[1].MinUs != 200 {
		t.Fatalf("expected min 200, got %d", entries[1].MinUs)
	}
	if entries[1].MaxUs != 600 {
		t.Fatalf("expected max 600, got %d", entries[1].MaxUs)
	}
}

func TestTrackerNoData(t *testing.T) {
	tracker := New()
	var r stubResolver
	c := &stubCollector{}
	if err := tracker.Read(c, r); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(tracker.Entries()) != 0 {
		t.Fatalf("expected 0 entries")
	}
}

func TestFormatLatencyEmpty(t *testing.T) {
	got := FormatLatency(nil, 5*time.Second)
	if got != "(no latency data)" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatLatency(t *testing.T) {
	entries := []Record{
		{Source: "frontend", Destination: "api", AvgUs: 1200, P95Us: 2500, P99Us: 5000, MaxUs: 8000, Samples: 42},
	}
	out := FormatLatency(entries, 10*time.Second)
	if !strings.Contains(out, "frontend → api") {
		t.Fatalf("missing pair label: %s", out)
	}
	if !strings.Contains(out, "1.20ms") {
		t.Fatalf("missing avg: %s", out)
	}
	if !strings.Contains(out, "2.50ms") {
		t.Fatalf("missing p95: %s", out)
	}
	if !strings.Contains(out, "5.00ms") {
		t.Fatalf("missing p99: %s", out)
	}
	if !strings.Contains(out, "42") {
		t.Fatalf("missing sample count: %s", out)
	}
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		us   float64
		want string
	}{
		{500, "500µs"},
		{1500, "1.50ms"},
		{1_500_000, "1.50s"},
		{0, "0µs"},
	}
	for _, c := range cases {
		got := FormatDuration(c.us)
		if got != c.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", c.us, got, c.want)
		}
	}
}

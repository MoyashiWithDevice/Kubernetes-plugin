package aggregator

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	detectivev1 "Kubernetes-plugin/api/detective/v1"
)

type nodeSnapshot struct {
	snap      *detectivev1.AgentSnapshot
	updatedAt time.Time
}

type Store struct {
	mu    sync.RWMutex
	nodes map[string]*nodeSnapshot
}

func NewStore() *Store {
	return &Store{
		nodes: make(map[string]*nodeSnapshot),
	}
}

func (s *Store) Update(nodeName string, snap *detectivev1.AgentSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes[nodeName] = &nodeSnapshot{
		snap:      snap,
		updatedAt: time.Now(),
	}
}

func (s *Store) Nodes() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var names []string
	for n := range s.nodes {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func (s *Store) LastUpdate() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest time.Time
	for _, ns := range s.nodes {
		if ns.updatedAt.After(latest) {
			latest = ns.updatedAt
		}
	}
	return latest
}

func resolveIP(ip []byte) string {
	if len(ip) == 4 {
		return net.IP(ip).String()
	}
	return fmt.Sprintf("%x", ip)
}

func (s *Store) GetFlows() *detectivev1.FlowList {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var flows []*detectivev1.FlowEvent
	for _, ns := range s.nodes {
		flows = append(flows, ns.snap.Flows...)
	}
	return &detectivev1.FlowList{Flows: flows}
}

func (s *Store) GetTop() *detectivev1.TopTalkerList {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type pair struct {
		tx, rx uint64
	}
	pairs := make(map[string]*pair)

	for _, ns := range s.nodes {
		for _, t := range ns.snap.Throughput {
			src := resolveIP(t.SrcIp)
			dst := resolveIP(t.DstIp)
			pk := src + "\x00" + dst
			p, ok := pairs[pk]
			if !ok {
				p = &pair{}
				pairs[pk] = p
			}
			p.tx += t.TxBytes
			p.rx += t.RxBytes
		}
	}

	var talkers []*detectivev1.TopTalker
	for pk, p := range pairs {
		parts := strings.SplitN(pk, "\x00", 2)
		src, dst := parts[0], ""
		if len(parts) == 2 {
			dst = parts[1]
		}
		talkers = append(talkers, &detectivev1.TopTalker{
			Source:      src,
			Destination: dst,
			TxBytes:     p.tx,
			RxBytes:     p.rx,
			TotalBytes:  p.tx + p.rx,
		})
	}

	sort.Slice(talkers, func(i, j int) bool {
		return talkers[i].TotalBytes > talkers[j].TotalBytes
	})

	return &detectivev1.TopTalkerList{Talkers: talkers}
}

func (s *Store) GetRetrans() *detectivev1.RetransList {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type agg struct {
		count uint64
	}
	pairs := make(map[string]*agg)

	for _, ns := range s.nodes {
		for _, r := range ns.snap.Retrans {
			src := resolveIP(r.SrcIp)
			dst := resolveIP(r.DstIp)
			pk := src + "\x00" + dst
			a, ok := pairs[pk]
			if !ok {
				a = &agg{}
				pairs[pk] = a
			}
			a.count += r.Count
		}
	}

	var records []*detectivev1.RetransRecord
	for pk, a := range pairs {
		parts := strings.SplitN(pk, "\x00", 2)
		src, dst := parts[0], ""
		if len(parts) == 2 {
			dst = parts[1]
		}
		records = append(records, &detectivev1.RetransRecord{
			Source:      src,
			Destination: dst,
			Count:       a.count,
		})
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].Count > records[j].Count
	})

	return &detectivev1.RetransList{Records: records}
}

func (s *Store) GetLatency() *detectivev1.LatencyList {
	s.mu.RLock()
	defer s.mu.RUnlock()

	const histBuckets = 27

	type agg struct {
		sumUs uint64
		count uint64
		minUs uint32
		maxUs uint32
		hist  [histBuckets]uint32
	}
	pairs := make(map[string]*agg)

	for _, ns := range s.nodes {
		for _, r := range ns.snap.Rtt {
			if r.Count == 0 {
				continue
			}
			src := resolveIP(r.SrcIp)
			dst := resolveIP(r.DstIp)
			pk := src + "\x00" + dst
			a, ok := pairs[pk]
			if !ok {
				a = &agg{minUs: r.MinUs, maxUs: r.MaxUs}
				pairs[pk] = a
			}
			a.sumUs += r.SumUs
			a.count += r.Count
			if r.MinUs > 0 && (a.minUs == 0 || r.MinUs < a.minUs) {
				a.minUs = r.MinUs
			}
			if r.MaxUs > a.maxUs {
				a.maxUs = r.MaxUs
			}
			for i := 0; i < histBuckets && i < len(r.Hist); i++ {
				a.hist[i] += r.Hist[i]
			}
		}
	}

	var records []*detectivev1.LatencyRecord
	for pk, a := range pairs {
		parts := strings.SplitN(pk, "\x00", 2)
		src, dst := parts[0], ""
		if len(parts) == 2 {
			dst = parts[1]
		}
		p95 := percentileFromHist(a.hist[:], a.count, 0.95)
		p99 := percentileFromHist(a.hist[:], a.count, 0.99)
		if a.maxUs > 0 {
			if p95 > float64(a.maxUs) {
				p95 = float64(a.maxUs)
			}
			if p99 > float64(a.maxUs) {
				p99 = float64(a.maxUs)
			}
		}
		var avgUs float64
		if a.count > 0 {
			avgUs = float64(a.sumUs) / float64(a.count)
		}
		records = append(records, &detectivev1.LatencyRecord{
			Source:      src,
			Destination: dst,
			AvgUs:       avgUs,
			P95Us:       p95,
			P99Us:       p99,
			MinUs:       a.minUs,
			MaxUs:       a.maxUs,
			Samples:     a.count,
		})
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].P95Us > records[j].P95Us
	})

	return &detectivev1.LatencyList{Records: records}
}

func (s *Store) GetDNS() *detectivev1.DNSList {
	s.mu.RLock()
	defer s.mu.RUnlock()

	const histBuckets = 27

	type agg struct {
		sumUs uint64
		count uint64
		minUs uint32
		maxUs uint32
		hist  [histBuckets]uint32
	}
	pairs := make(map[string]*agg)

	for _, ns := range s.nodes {
		for _, d := range ns.snap.Dns {
			if d.Count == 0 {
				continue
			}
			src := resolveIP(d.SrcIp)
			dst := resolveIP(d.DstIp)
			pk := src + "\x00" + dst
			a, ok := pairs[pk]
			if !ok {
				a = &agg{minUs: d.MinUs, maxUs: d.MaxUs}
				pairs[pk] = a
			}
			a.sumUs += d.SumUs
			a.count += d.Count
			if d.MinUs > 0 && (a.minUs == 0 || d.MinUs < a.minUs) {
				a.minUs = d.MinUs
			}
			if d.MaxUs > a.maxUs {
				a.maxUs = d.MaxUs
			}
			for i := 0; i < histBuckets && i < len(d.Hist); i++ {
				a.hist[i] += d.Hist[i]
			}
		}
	}

	var records []*detectivev1.DNSRecord
	for pk, a := range pairs {
		parts := strings.SplitN(pk, "\x00", 2)
		src, dst := parts[0], ""
		if len(parts) == 2 {
			dst = parts[1]
		}
		p95 := percentileFromHist(a.hist[:], a.count, 0.95)
		p99 := percentileFromHist(a.hist[:], a.count, 0.99)
		if a.maxUs > 0 {
			if p95 > float64(a.maxUs) {
				p95 = float64(a.maxUs)
			}
			if p99 > float64(a.maxUs) {
				p99 = float64(a.maxUs)
			}
		}
		var avgUs float64
		if a.count > 0 {
			avgUs = float64(a.sumUs) / float64(a.count)
		}
		records = append(records, &detectivev1.DNSRecord{
			Source:      src,
			Destination: dst,
			AvgUs:       avgUs,
			P95Us:       p95,
			P99Us:       p99,
			MinUs:       a.minUs,
			MaxUs:       a.maxUs,
			Queries:     a.count,
		})
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].P95Us > records[j].P95Us
	})

	return &detectivev1.DNSList{Records: records}
}

func percentileFromHist(hist []uint32, count uint64, p float64) float64 {
	if count == 0 || p <= 0 {
		return 0
	}
	if p > 1 {
		p = 1
	}
	target := uint64(float64(count) * p)
	if target == 0 {
		target = 1
	}
	if target > count {
		target = count
	}

	var cum uint64
	for i, n := range hist {
		cum += uint64(n)
		if cum >= target {
			lo := uint64(1) << uint(i)
			if i == 0 {
				return float64(lo)
			}
			hi := uint64(1) << uint(i+1)
			return float64(lo+hi) / 2
		}
	}
	last := len(hist) - 1
	return float64(uint64(1) << uint(last))
}

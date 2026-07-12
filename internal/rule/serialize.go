package rule

import (
	"encoding/json"
	"net"

	"Kubernetes-plugin/internal/flow"
)

func SerializeContext(ctx *Context) ([]byte, error) {
	return json.Marshal(ctx)
}

func NewContextFromFlows(events []flow.FlowEvent) *Context {
	flows := make([]FlowEntry, len(events))
	for i, ev := range events {
		flows[i] = FlowEntry{
			SrcIP:   ev.SrcIP.String(),
			DstIP:   ev.DstIP.String(),
			SrcPort: ev.SrcPort,
			DstPort: ev.DstPort,
			PID:     ev.PID,
			Comm:    ev.Comm,
		}
	}
	return &Context{Flows: flows}
}

func NewContextFromThroughput(collector interface {
	ReadThroughput(func(flow.ThroughputKey, flow.ThroughputVal) error) error
}) *Context {
	ctx := &Context{}
	_ = collector.ReadThroughput(func(key flow.ThroughputKey, val flow.ThroughputVal) error {
		ctx.Throughputs = append(ctx.Throughputs, ThroughputEntry{
			SrcIP:   net.IP(key.SrcIP[:]).String(),
			DstIP:   net.IP(key.DstIP[:]).String(),
			SrcPort: key.SrcPort,
			DstPort: key.DstPort,
			TxBytes: val.TxBytes,
			RxBytes: val.RxBytes,
		})
		return nil
	})
	return ctx
}

func NewContextFromRetrans(collector interface {
	ReadRetrans(func(flow.RetransKey, flow.RetransVal) error) error
}) *Context {
	ctx := &Context{}
	_ = collector.ReadRetrans(func(key flow.RetransKey, val flow.RetransVal) error {
		ctx.Retrans = append(ctx.Retrans, RetransEntry{
			SrcIP:   net.IP(key.SrcIP[:]).String(),
			DstIP:   net.IP(key.DstIP[:]).String(),
			SrcPort: key.SrcPort,
			DstPort: key.DstPort,
			Count:   val.Count,
		})
		return nil
	})
	return ctx
}

func NewContextFromRTT(collector interface {
	ReadRTT(func(flow.RTTKey, flow.RTTVal) error) error
}) *Context {
	ctx := &Context{}
	_ = collector.ReadRTT(func(key flow.RTTKey, val flow.RTTVal) error {
		var avgUs uint64
		if val.Count > 0 {
			avgUs = val.SumUs / val.Count
		}
		ctx.RTT = append(ctx.RTT, RTTEntry{
			SrcIP:   net.IP(key.SrcIP[:]).String(),
			DstIP:   net.IP(key.DstIP[:]).String(),
			SrcPort: key.SrcPort,
			DstPort: key.DstPort,
			AvgUs:   avgUs,
			MinUs:   val.MinUs,
			MaxUs:   val.MaxUs,
			Count:   val.Count,
		})
		return nil
	})
	return ctx
}

func MergeContexts(a, b *Context) *Context {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	return &Context{
		Throughputs: append(a.Throughputs, b.Throughputs...),
		Retrans:     append(a.Retrans, b.Retrans...),
		RTT:         append(a.RTT, b.RTT...),
		Flows:       append(a.Flows, b.Flows...),
	}
}

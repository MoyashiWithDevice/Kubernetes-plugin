#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

char LICENSE[] SEC("license") = "Dual BSD/GPL";

struct flow_event {
	__u32 src_ip;
	__u32 dst_ip;
	__u16 src_port;
	__u16 dst_port;
	__u32 pid;
	char comm[16];
};

struct {
	__uint(type, BPF_MAP_TYPE_RINGBUF);
	__uint(max_entries, 4096);
} events SEC(".maps");

struct throughput_key_t {
	__u32 src_ip;
	__u32 dst_ip;
	__u16 src_port;
	__u16 dst_port;
};

struct throughput_val_t {
	__u64 tx_bytes;
	__u64 rx_bytes;
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, 65536);
	__type(key, struct throughput_key_t);
	__type(value, struct throughput_val_t);
} throughput_map SEC(".maps");

struct retrans_key_t {
	__u32 src_ip;
	__u32 dst_ip;
	__u16 src_port;
	__u16 dst_port;
};

struct retrans_val_t {
	__u64 count;
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, 65536);
	__type(key, struct retrans_key_t);
	__type(value, struct retrans_val_t);
} retrans_map SEC(".maps");

/* Log2 histogram: bucket i covers [2^i, 2^(i+1)) microseconds. */
#define RTT_HIST_BUCKETS 27

struct rtt_key_t {
	__u32 src_ip;
	__u32 dst_ip;
	__u16 src_port;
	__u16 dst_port;
};

struct rtt_val_t {
	__u64 sum_us;
	__u64 count;
	__u32 min_us;
	__u32 max_us;
	__u32 hist[RTT_HIST_BUCKETS];
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, 65536);
	__type(key, struct rtt_key_t);
	__type(value, struct rtt_val_t);
} rtt_map SEC(".maps");

static __always_inline int rtt_log2_u32(__u32 v)
{
	int r = 0;

	if (v == 0)
		return 0;
	if (v >= 0x10000) {
		r += 16;
		v >>= 16;
	}
	if (v >= 0x100) {
		r += 8;
		v >>= 8;
	}
	if (v >= 0x10) {
		r += 4;
		v >>= 4;
	}
	if (v >= 0x4) {
		r += 2;
		v >>= 2;
	}
	if (v >= 0x2)
		r += 1;
	return r;
}

/* ── DNS (UDP/53) ──────────────────────────────────────────── */

#define DNS_PORT 53
#define DNS_NAME_LEN 64

struct dns_query_key_t {
	__u32 src_ip;
	__u32 dst_ip;
	__u16 src_port;
	__u16 dst_port;
};

struct dns_query_val_t {
	__u64 ts_ns;
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, 65536);
	__type(key, struct dns_query_key_t);
	__type(value, struct dns_query_val_t);
} dns_query_map SEC(".maps");

struct dns_stats_key_t {
	__u32 src_ip;
	__u32 dst_ip;
};

struct dns_stats_val_t {
	__u64 count;
	__u64 sum_us;
	__u32 min_us;
	__u32 max_us;
	__u32 hist[RTT_HIST_BUCKETS];
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, 65536);
	__type(key, struct dns_stats_key_t);
	__type(value, struct dns_stats_val_t);
} dns_stats_map SEC(".maps");

struct dns_event_t {
	__u64 latency_ns;
	__u32 src_ip;
	__u32 dst_ip;
	__u32 pid;
	__u16 src_port;
	__u16 dst_port;
	char comm[16];
	char query[DNS_NAME_LEN];
};

struct {
	__uint(type, BPF_MAP_TYPE_RINGBUF);
	__uint(max_entries, 4096);
} dns_events SEC(".maps");

SEC("kprobe/udp_sendmsg")
int BPF_KPROBE(kprobe__udp_sendmsg, struct sock *sk)
{
	__u16 dport;
	bpf_core_read(&dport, sizeof(dport), &sk->__sk_common.skc_dport);
	if (dport != __builtin_bswap16(DNS_PORT))
		return 0;

	struct dns_query_key_t key = {};
	bpf_core_read(&key.src_ip, sizeof(key.src_ip),
		       &sk->__sk_common.skc_rcv_saddr);
	bpf_core_read(&key.dst_ip, sizeof(key.dst_ip),
		       &sk->__sk_common.skc_daddr);
	bpf_core_read(&key.src_port, sizeof(key.src_port),
		       &sk->__sk_common.skc_num);
	key.dst_port = DNS_PORT;

	struct dns_query_val_t qv = {};
	qv.ts_ns = bpf_ktime_get_ns();
	bpf_map_update_elem(&dns_query_map, &key, &qv, BPF_ANY);

	struct dns_event_t *ev;
	ev = bpf_ringbuf_reserve(&dns_events, sizeof(*ev), 0);
	if (!ev)
		return 0;
	ev->src_ip = key.src_ip;
	ev->dst_ip = key.dst_ip;
	ev->src_port = key.src_port;
	ev->dst_port = key.dst_port;
	ev->latency_ns = 0;
	ev->pid = bpf_get_current_pid_tgid() >> 32;
	bpf_get_current_comm(ev->comm, sizeof(ev->comm));
	__builtin_memset(ev->query, 0, sizeof(ev->query));
	bpf_ringbuf_submit(ev, 0);
	return 0;
}

SEC("kprobe/udp_rcv")
int BPF_KPROBE(kprobe__udp_rcv, struct sk_buff *skb)
{
	__u16 transport_header;
	bpf_core_read(&transport_header, sizeof(transport_header),
		       &skb->transport_header);

	unsigned char *head;
	bpf_core_read(&head, sizeof(head), &skb->head);

	/* udphdr: source (offset 0), dest (offset 2) */
	__u16 sport;
	bpf_probe_read_kernel(&sport, sizeof(sport), head + transport_header);
	if (sport != __builtin_bswap16(DNS_PORT))
		return 0;

	__u16 dport;
	bpf_probe_read_kernel(&dport, sizeof(dport),
			      head + transport_header + 2);

	/* iphdr: saddr (offset 12), daddr (offset 16) */
	__u16 network_header;
	bpf_core_read(&network_header, sizeof(network_header),
		       &skb->network_header);

	__u32 src_ip;
	bpf_probe_read_kernel(&src_ip, sizeof(src_ip),
			      head + network_header + 12);
	__u32 dst_ip;
	bpf_probe_read_kernel(&dst_ip, sizeof(dst_ip),
			      head + network_header + 16);

	/* Map response back to the original query key. */
	struct dns_query_key_t qkey = {};
	qkey.src_ip = dst_ip;
	qkey.dst_ip = src_ip;
	qkey.src_port = dport;
	qkey.dst_port = DNS_PORT;

	struct dns_query_val_t *qv = bpf_map_lookup_elem(&dns_query_map,
							 &qkey);
	if (!qv)
		return 0;

	__u64 latency_ns = bpf_ktime_get_ns() - qv->ts_ns;
	bpf_map_delete_elem(&dns_query_map, &qkey);

	/* Aggregate stats by src→dst pair. */
	__u64 lat_us = latency_ns / 1000;
	struct dns_stats_key_t skey = {};
	skey.src_ip = qkey.src_ip;
	skey.dst_ip = qkey.dst_ip;

	struct dns_stats_val_t *sv = bpf_map_lookup_elem(&dns_stats_map,
							 &skey);
	if (sv) {
		__sync_fetch_and_add(&sv->count, 1);
		__sync_fetch_and_add(&sv->sum_us, lat_us);
		if (lat_us < sv->min_us || sv->min_us == 0)
			sv->min_us = lat_us;
		if (lat_us > sv->max_us)
			sv->max_us = lat_us;
		int b = rtt_log2_u32(lat_us);
		if (b >= RTT_HIST_BUCKETS)
			b = RTT_HIST_BUCKETS - 1;
		__sync_fetch_and_add(&sv->hist[b], 1);
	} else {
		struct dns_stats_val_t nsv = {};
		nsv.count = 1;
		nsv.sum_us = lat_us;
		nsv.min_us = lat_us;
		nsv.max_us = lat_us;
		int b = rtt_log2_u32(lat_us);
		if (b >= RTT_HIST_BUCKETS)
			b = RTT_HIST_BUCKETS - 1;
		nsv.hist[b] = 1;
		bpf_map_update_elem(&dns_stats_map, &skey, &nsv, BPF_ANY);
	}

	/* Read DNS query name from response question section.
	 * DNS data = UDP payload (transport_header + 8).
	 * Query name starts at DNS offset 12 (after DNS header). */
	char name[DNS_NAME_LEN] = {};
	void *name_ptr = (void *)(head + transport_header + 8 + 12);
	bpf_probe_read_kernel(name, sizeof(name) - 1, name_ptr);

	struct dns_event_t *ev;
	ev = bpf_ringbuf_reserve(&dns_events, sizeof(*ev), 0);
	if (!ev)
		return 0;
	ev->src_ip = qkey.src_ip;
	ev->dst_ip = qkey.dst_ip;
	ev->src_port = qkey.src_port;
	ev->dst_port = qkey.dst_port;
	ev->latency_ns = latency_ns;
	ev->pid = bpf_get_current_pid_tgid() >> 32;
	bpf_get_current_comm(ev->comm, sizeof(ev->comm));
	__builtin_memcpy(ev->query, name, sizeof(ev->query) - 1);
	bpf_ringbuf_submit(ev, 0);
	return 0;
}

SEC("kprobe/tcp_connect")
int kprobe__tcp_connect(struct pt_regs *ctx)
{
	struct sock *sk = (typeof(sk))PT_REGS_PARM1(ctx);
	struct flow_event *ev;

	ev = bpf_ringbuf_reserve(&events, sizeof(*ev), 0);
	if (!ev)
		return 0;

	bpf_core_read(&ev->src_ip, sizeof(ev->src_ip), &sk->__sk_common.skc_rcv_saddr);
	bpf_core_read(&ev->dst_ip, sizeof(ev->dst_ip), &sk->__sk_common.skc_daddr);
	bpf_core_read(&ev->src_port, sizeof(ev->src_port), &sk->__sk_common.skc_num);
	bpf_core_read(&ev->dst_port, sizeof(ev->dst_port), &sk->__sk_common.skc_dport);

	ev->pid = bpf_get_current_pid_tgid() >> 32;
	bpf_get_current_comm(ev->comm, sizeof(ev->comm));

	bpf_ringbuf_submit(ev, 0);
	return 0;
}

SEC("kprobe/tcp_sendmsg")
int kprobe__tcp_sendmsg(struct pt_regs *ctx)
{
	struct sock *sk = (typeof(sk))PT_REGS_PARM1(ctx);
	size_t size = (size_t)PT_REGS_PARM3(ctx);

	struct throughput_key_t key = {};
	bpf_core_read(&key.src_ip, sizeof(key.src_ip), &sk->__sk_common.skc_rcv_saddr);
	bpf_core_read(&key.dst_ip, sizeof(key.dst_ip), &sk->__sk_common.skc_daddr);
	bpf_core_read(&key.src_port, sizeof(key.src_port), &sk->__sk_common.skc_num);
	bpf_core_read(&key.dst_port, sizeof(key.dst_port), &sk->__sk_common.skc_dport);

	struct throughput_val_t *val = bpf_map_lookup_elem(&throughput_map, &key);
	if (val) {
		__sync_fetch_and_add(&val->tx_bytes, size);
	} else {
		struct throughput_val_t new_val = {};
		new_val.tx_bytes = size;
		bpf_map_update_elem(&throughput_map, &key, &new_val, BPF_ANY);
	}
	return 0;
}

SEC("kprobe/tcp_cleanup_rbuf")
int kprobe__tcp_cleanup_rbuf(struct pt_regs *ctx)
{
	struct sock *sk = (typeof(sk))PT_REGS_PARM1(ctx);
	int copied = (int)PT_REGS_PARM2(ctx);

	if (copied <= 0)
		return 0;

	struct throughput_key_t key = {};
	bpf_core_read(&key.src_ip, sizeof(key.src_ip), &sk->__sk_common.skc_rcv_saddr);
	bpf_core_read(&key.dst_ip, sizeof(key.dst_ip), &sk->__sk_common.skc_daddr);
	bpf_core_read(&key.src_port, sizeof(key.src_port), &sk->__sk_common.skc_num);
	bpf_core_read(&key.dst_port, sizeof(key.dst_port), &sk->__sk_common.skc_dport);

	struct throughput_val_t *val = bpf_map_lookup_elem(&throughput_map, &key);
	if (val) {
		__sync_fetch_and_add(&val->rx_bytes, copied);
	} else {
		struct throughput_val_t new_val = {};
		new_val.rx_bytes = copied;
		bpf_map_update_elem(&throughput_map, &key, &new_val, BPF_ANY);
	}
	return 0;
}

SEC("kprobe/tcp_retransmit_skb")
int kprobe__tcp_retransmit_skb(struct pt_regs *ctx)
{
	struct sock *sk = (typeof(sk))PT_REGS_PARM1(ctx);

	struct retrans_key_t key = {};
	bpf_core_read(&key.src_ip, sizeof(key.src_ip), &sk->__sk_common.skc_rcv_saddr);
	bpf_core_read(&key.dst_ip, sizeof(key.dst_ip), &sk->__sk_common.skc_daddr);
	bpf_core_read(&key.src_port, sizeof(key.src_port), &sk->__sk_common.skc_num);
	bpf_core_read(&key.dst_port, sizeof(key.dst_port), &sk->__sk_common.skc_dport);

	struct retrans_val_t *val = bpf_map_lookup_elem(&retrans_map, &key);
	if (val) {
		__sync_fetch_and_add(&val->count, 1);
	} else {
		struct retrans_val_t new_val = {};
		new_val.count = 1;
		bpf_map_update_elem(&retrans_map, &key, &new_val, BPF_ANY);
	}
	return 0;
}

SEC("kprobe/tcp_rcv_established")
int kprobe__tcp_rcv_established(struct pt_regs *ctx)
{
	struct sock *sk = (typeof(sk))PT_REGS_PARM1(ctx);
	struct tcp_sock *tp = (struct tcp_sock *)sk;
	__u32 srtt = 0;

	/* srtt_us is stored as smoothed RTT << 3. */
	bpf_core_read(&srtt, sizeof(srtt), &tp->srtt_us);
	__u32 rtt_us = srtt >> 3;
	if (rtt_us == 0)
		return 0;

	struct rtt_key_t key = {};
	bpf_core_read(&key.src_ip, sizeof(key.src_ip), &sk->__sk_common.skc_rcv_saddr);
	bpf_core_read(&key.dst_ip, sizeof(key.dst_ip), &sk->__sk_common.skc_daddr);
	bpf_core_read(&key.src_port, sizeof(key.src_port), &sk->__sk_common.skc_num);
	bpf_core_read(&key.dst_port, sizeof(key.dst_port), &sk->__sk_common.skc_dport);

	int bucket = rtt_log2_u32(rtt_us);
	if (bucket >= RTT_HIST_BUCKETS)
		bucket = RTT_HIST_BUCKETS - 1;

	struct rtt_val_t *val = bpf_map_lookup_elem(&rtt_map, &key);
	if (val) {
		__sync_fetch_and_add(&val->sum_us, rtt_us);
		__sync_fetch_and_add(&val->count, 1);
		__sync_fetch_and_add(&val->hist[bucket], 1);
		if (rtt_us < val->min_us || val->min_us == 0)
			val->min_us = rtt_us;
		if (rtt_us > val->max_us)
			val->max_us = rtt_us;
	} else {
		struct rtt_val_t new_val = {};
		new_val.sum_us = rtt_us;
		new_val.count = 1;
		new_val.min_us = rtt_us;
		new_val.max_us = rtt_us;
		new_val.hist[bucket] = 1;
		bpf_map_update_elem(&rtt_map, &key, &new_val, BPF_ANY);
	}
	return 0;
}

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

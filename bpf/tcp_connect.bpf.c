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

// SPDX-License-Identifier: GPL-2.0
// Cgroup LSM: capture sockets may receive, but cannot transmit packets.
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_tracing.h>

struct sock_common { unsigned short skc_family; } __attribute__((preserve_access_index));
struct sock { struct sock_common __sk_common; } __attribute__((preserve_access_index));
struct socket { short type; struct sock *sk; } __attribute__((preserve_access_index));
struct msghdr;

SEC("lsm_cgroup/socket_create")
int BPF_PROG(adcap_create, int family, int type, int protocol, int kern)
{
    (void)ctx; (void)protocol; (void)kern;
    // Deny raw IP, obsolete SOCK_PACKET, and AF_XDP injection paths.
    if ((family == 2 || family == 10) && type == 3)
        return 0;
    if (family == 44 || type == 10)
        return 0;
    return 1;
}

SEC("lsm_cgroup/socket_sendmsg")
int BPF_PROG(adcap_send, struct socket *sock, struct msghdr *msg, int size)
{
    (void)ctx; (void)msg; (void)size;
    unsigned short family = BPF_CORE_READ(sock, sk, __sk_common.skc_family);
    if (family == 17 || family == 44)
        return 0;
    if ((family == 2 || family == 10) && BPF_CORE_READ(sock, type) == 3)
        return 0;
    return 1;
}

SEC("lsm_cgroup/socket_setsockopt")
int BPF_PROG(adcap_setopt, struct socket *sock, int level, int optname)
{
    (void)ctx; (void)sock;
    // No transmit ring, transparent/nonlocal source binding, or packet TX bypass.
    if (level == 263 && optname == 13) // SOL_PACKET / PACKET_TX_RING
        return 0;
    if (level == 0 && (optname == 19 || optname == 15))
        return 0; // IP_TRANSPARENT / IP_FREEBIND
    if (level == 41 && (optname == 75 || optname == 78))
        return 0; // IPV6_TRANSPARENT / IPV6_FREEBIND
    return 1;
}
char LICENSE[] SEC("license") = "GPL";

// Drop unrelated ingress before packet sockets/tcpdump receive a copy. This
// profile deliberately supports the platform's static IPv4 Ethernet networks.
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __uint(map_flags, BPF_F_RDONLY_PROG);
    __type(key, __u32);
    __type(value, __u32);
} rx_config SEC(".maps");

SEC("xdp")
int adcap_ingress(struct xdp_md *ctx)
{
    unsigned char *data = (void *)(long)ctx->data;
    unsigned char *end = (void *)(long)ctx->data_end;
    __u32 key = 0, *own_ip = bpf_map_lookup_elem(&rx_config, &key);
    if (!own_ip || !*own_ip || data + 14 > end)
        return XDP_DROP;
    __u16 protocol = ((__u16)data[12] << 8) | data[13];
    __u32 destination;
    if (protocol == 0x0800) {
        if (data + 34 > end || data[14] >> 4 != 4 || (data[14] & 15) < 5)
            return XDP_DROP;
        __builtin_memcpy(&destination, data + 30, 4);
        return destination == *own_ip ? XDP_PASS : XDP_DROP;
    }
    if (protocol == 0x0806) {
        // Ethernet/IPv4 ARP, target protocol address must be this instance.
        if (data + 42 > end || data[14] != 0 || data[15] != 1 ||
            data[16] != 8 || data[17] != 0 || data[18] != 6 || data[19] != 4 ||
            data[20] != 0 || (data[21] != 1 && data[21] != 2))
            return XDP_DROP;
        __builtin_memcpy(&destination, data + 38, 4);
        return destination == *own_ip ? XDP_PASS : XDP_DROP;
    }
    // Includes broadcast/multicast IPv6, VLANs, and unknown EtherTypes.
    return XDP_DROP;
}

# Direct tcpdump with kernel-enforced transmit and receive boundaries

Production guarded mode runs the ordinary `tcpdump` executable inside a challenge
container. No capture helper, capture sidecar, wrapper, or running userspace broker
is needed. The controller loads host kernel policies before releasing the original
challenge command. The default capability list excludes `NET_RAW`; guarded mode
adds it only to containers created with the immutable startup gate.

The implementation combines two host-controlled policies:

- A cgroup BPF LSM policy permits capture sockets but denies raw-packet
  transmission, including from root or a replacement executable.
- An XDP ingress policy on the instance's external interface drops frames
  before packet sockets receive a copy. It passes IPv4 packets addressed to
  the instance's configured address and Ethernet/IPv4 ARP requests/replies
  targeting that address. Other IPv4 destinations, broadcast/multicast IP
  payloads, IPv6, and unsupported Ethernet types are dropped.

The receive policy has a fixed per-interface IPv4 configuration. Its map is
frozen before attachment; it cannot be updated through a participant option.
This profile intentionally targets the platform's static IPv4 Ethernet networks.
It does not support DHCP, IPv6, multicast-dependent services, or arbitrary
layer-two protocols. Those require a separately reviewed profile.

The send policy denies transmission on `AF_PACKET`, raw IPv4/IPv6 sockets,
obsolete `SOCK_PACKET`, `AF_XDP`, `PACKET_TX_RING`, and transparent/nonlocal IP
source binding. Normal TCP/UDP and packet capture still work for the instance.
This is mandatory kernel enforcement, not a restriction on the commands a
participant types. The XDP receive gate is independent of tcpdump's own filters.

`NET_ADMIN`, `BPF`, `PERFMON`, and `SYS_ADMIN` must remain absent from challenge
containers. Keep `no-new-privileges`, Docker's default seccomp policy, private
network/PID namespaces, and the cgroup filesystem read-only. Do not expose the
Docker socket, host namespace handles, or host filesystem to challenges.
The trusted one-shot host loader needs privileges; those belong only to the
operator and are never passed to participants.

## Participant usage

Once guarded capture is deployed, teams can use the standard commands directly:

```bash
# Live incoming traffic. Add a service-port filter as needed.
tcpdump -p -Q in -i eth0 -nn

# Save incoming traffic, then inspect it later.
tcpdump -p -Q in -i eth0 -w /tmp/incoming.pcap
tcpdump -nn -r /tmp/incoming.pcap
```

`-Q in` selects capture direction; it is not the injection protection. The kernel
send policy is what denies injection, even if the participant changes arguments,
replaces tcpdump, or runs their own packet-sending program as root.
The policy does not prevent ordinary TCP/UDP floods or every form of network
abuse. Existing isolation, access rules, limits, and updated host kernels remain
necessary. Passive captures can still consume disk or CPU inside the container.

A participant can still capture traffic they send themselves, incoming traffic
addressed to their own instance, and local loopback traffic. The receive boundary
prevents unrelated frames from other instances being delivered to the capture
socket. It does not hide secrets deliberately sent to this instance, encrypt
checker traffic, or prevent an attacker who already controls the victim instance
from observing that victim's traffic.

## Reproduce the isolated test

Requires a Linux host with cgroup v2, active `bpf` LSM, BTF, Docker, Python 3,
clang with the BPF backend, gcc, libbpf development files, and ldd:

```bash
python3 deploy/security/direct-capture/test.py
```

The test builds a disposable HTTP example image and two instances on a temporary
shared Docker bridge, plus a third container issuing checker-style updates over
that bridge. Only the disposable target's cgroup and network interface
receive policies. It verifies:

- Before the receive gate, broad root tcpdump can see foreign broadcast,
  multicast, deliberately flooded foreign-destination IPv4, ARP, IPv6, and VLAN
  payloads. Every negative payload must appear in this baseline, proving that
  the later absence is caused by the guard.
- With the gate, those payloads and the other instance's normal checker update
  are absent from captures on both `eth0` (promiscuous capture requested) and
  `any` (cooked capture), without a tcpdump-level address filter.
- The target's own checker flag update remains visible. Checker-style writes to
  both instances and ordinary HTTP requests continue to succeed.
- Injection as root fails through `send`, `sendto`, `sendmsg`, `sendmmsg`,
  `write`, `writev`, transmit rings, raw IP sockets, and transparent binding.
- Container-root attempts to remove XDP through rtnetlink are denied.
- Both policies detach and all temporary containers, networks, and image tags
  are removed. No live platform container or host cgroup root is modified.

The unguarded trusted fixture sender deliberately emits malformed or flooded
frames **only in the disposable network** to exercise the visibility boundary.
It does not represent capabilities granted to participant containers.

The test uses a temporary management container to run the privileged one-shot
loader. This is a testing mechanism; it does not capture or forward packets and
is not needed while participants run tcpdump.

## Deployment requirements

Configure a local rootful Linux Docker host with cgroup v2, kernel BTF, and
`bpf` in `/sys/kernel/security/lsm`. On hosts without BPF LSM, an operator must
enable it in the kernel boot configuration and reboot. The scripts do not change
boot settings automatically. Rootless Docker and user namespace remapping are
unsupported. The current profile requires one external `eth0`, a fixed IPv4
address, and challenge images that start as root (the application may then drop
privileges).

```bash
# Adds guarded capture to older env files, preserves secrets and explicit modes.
make setup-prod-env DOMAIN=ctf.example.com SCHEME=https CAPTURE_MODE=guarded
make preflight-prod-host
make up-prod-host
```

`make setup-prod-env` defaults a missing `CONTROLLER_CAPTURE_MODE` to `guarded`
and enforces `no-new-privileges:true`. An existing explicit `disabled` setting is
preserved unless `CAPTURE_MODE=guarded` is passed. `CAPTURE_MODE=disabled` keeps
PCAP reading available without live packet sockets. Never add `NET_RAW` manually.
Use the host-enforcement Compose overlay; the base stack alone lacks policy-loading
privileges and mounts.

The deployment builds a static startup executable into a trusted named volume.
A network-disabled, capability-free one-shot initializer populates that volume;
it exits before the controller starts. Each challenge mounts it read-only with image-to-volume copying disabled. An
empty or unlabelled volume cannot substitute an image-supplied fake gate. At
every boot, the executable waits on a private network-namespace abstract Unix
socket with a fresh in-memory nonce. The controller verifies the expected cgroup
programs, XDP program, frozen address map, interface count, instance identity,
and read-only mount before releasing that boot. It then execs the original
image entrypoint and arguments. No capture process remains. Image healthchecks
are disabled to prevent executing image code before guard attachment.

This covers fresh deploys, ordinary restarts, factory resets, and external Docker
restarts. The controller polls every two seconds for changed boot generations,
and verifies running instances again every thirty seconds. If it must repair a
running instance, it first restarts into the trusted gate. Policy errors stop the
instance or leave its application blocked; there is no unrestricted fallback.
Controller reconciliation preserves the writable layer of existing guarded
instances. Guard failures that stopped an instance require explicit controller
reconciliation/restart after the host issue is repaired.

The trusted controller alone receives `BPF`, `PERFMON`, `NET_ADMIN`, and `SYS_ADMIN`,
uses the host cgroup namespace, and mounts host cgroups plus read-only host process
metadata and Docker namespace handles. The namespace directory uses `rslave`
propagation so namespaces created after controller startup are visible. These
privileges are never granted to challenge containers. The controller is part of
the trusted control plane and already has Docker administration access. Its API
and Docker proxy must remain inaccessible from game networks.

Existing ungated instances need recreation to receive the immutable mount and
capability profile. Plan the first rollout during maintenance: controller startup
reconciliation recreates legacy instances, retaining their named state volumes
but losing writable-layer patches. Rebuild the examples to install tcpdump, and
preserve backups before rollout. Later guarded reconciliations preserve patches.
Changing the bootstrap volume name requires another controlled migration.

Verify production artifacts without touching the platform stack:

```bash
python3 scripts/test-capture-setup.py
./scripts/smoke-live-capture.sh
python3 deploy/security/direct-capture/test.py
```

The lifecycle smoke builds the production controller and startup images, creates
only randomly named disposable resources, and verifies live capture, root
injection denial, read-only startup code, patch-preserving recovery, blocked
startup after Docker restart, guard failure, and factory reset. The separate
receive-boundary test exercises two challenge instances and foreign frames.

Build the policy on the host with:

```bash
cd deploy/security/direct-capture
clang -O2 -g -target bpf -c capture.bpf.c -o capture.bpf.o
cc -O2 -Wall -Wextra policy.c -o policy -lbpf
```

The one-shot loader supports:

```text
policy attach CGROUP capture.bpf.o
policy detach CGROUP
policy check CGROUP capture.bpf.o
policy rx-attach NETNS_FILE INTERFACE INSTANCE_IPV4 capture.bpf.o
policy rx-detach NETNS_FILE INTERFACE
policy rx-check NETNS_FILE INTERFACE INSTANCE_IPV4 capture.bpf.o
```

The namespace file is a trusted host handle to the target container's namespace,
for example `/proc/<container-pid>/ns/net`. Receive attach refuses to replace an
existing XDP program. Receive detach refuses an unrelated program and uses the
old program FD to avoid detaching a replacement that raced the check.

Attach refuses to replace existing local cgroup LSM
policies and refuses the host cgroup root. Detach is an operator action: stop the
protected challenge containers before removing their policy.

The implementation follows the Linux kernel's
[BPF LSM documentation](https://docs.kernel.org/bpf/prog_lsm.html) and
[cgroup LSM selftests](https://github.com/torvalds/linux/blob/master/tools/testing/selftests/bpf/progs/lsm_cgroup.c).

The receive gate uses generic XDP, which executes before packet capture taps in
the [Linux receive path](https://github.com/torvalds/linux/blob/v6.12/net/core/dev.c).
A firewall or tc ingress drop alone is too late for this capture boundary.

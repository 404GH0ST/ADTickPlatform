#define _GNU_SOURCE
// SPDX-License-Identifier: GPL-2.0
// One-shot loader. Cgroup attachments remain in the kernel after this exits.
#include <bpf/bpf.h>
#include <bpf/libbpf.h>
#include <errno.h>
#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/stat.h>
#include <sys/vfs.h>
#include <sched.h>
#include <net/if.h>
#include <arpa/inet.h>
#include <linux/if_link.h>

static struct bpf_object *load_policy(const char *path, enum bpf_prog_type type)
{
    struct bpf_object *obj = bpf_object__open_file(path, NULL);
    if (libbpf_get_error(obj)) return NULL;
    struct bpf_program *p;
    bpf_object__for_each_program(p, obj)
        bpf_program__set_autoload(p, bpf_program__type(p) == type);
    if (bpf_object__load(obj)) { bpf_object__close(obj); return NULL; }
    return obj;
}
static int same_program(int expected, int actual)
{
    struct bpf_prog_info a = {}, b = {};
    __u32 size = sizeof(a);
    if (bpf_prog_get_info_by_fd(expected, &a, &size)) return 0;
    size = sizeof(b);
    if (bpf_prog_get_info_by_fd(actual, &b, &size)) return 0;
    return a.type == b.type && a.attach_btf_id == b.attach_btf_id &&
        !memcmp(a.tag, b.tag, BPF_TAG_SIZE) && !strcmp((char *)a.name, (char *)b.name);
}
static int check_cgroup(int cg, struct bpf_object *obj)
{
    __u32 ids[128];
    LIBBPF_OPTS(bpf_prog_query_opts, query,
        .query_flags = BPF_F_QUERY_EFFECTIVE, .prog_ids = ids, .prog_cnt = 128);
    // Include ancestors: no additional cgroup LSM may change allow semantics.
    if (bpf_prog_query_opts(cg, BPF_LSM_CGROUP, &query) || query.prog_cnt != 3) return 1;
    struct bpf_program *p;
    unsigned int expected = 0;
    bpf_object__for_each_program(p, obj) {
        if (bpf_program__type(p) != BPF_PROG_TYPE_LSM) continue;
        expected++;
        int found = 0;
        for (__u32 i = 0; i < query.prog_cnt; i++) {
            int fd = bpf_prog_get_fd_by_id(ids[i]);
            if (fd < 0) return 1;
            found = same_program(bpf_program__fd(p), fd);
            close(fd);
            if (found) break;
        }
        if (!found) return 1;
    }
    return expected == 3 ? 0 : 1;
}
static int check_rx_config(int fd, __u32 expected)
{
    struct bpf_prog_info info = {};
    __u32 ids[16], size = sizeof(info);
    info.nr_map_ids = 16; info.map_ids = (__u64)(unsigned long)ids;
    if (bpf_prog_get_info_by_fd(fd, &info, &size) || info.nr_map_ids != 1) { fprintf(stderr,"unexpected receive maps: %u: %s\n",info.nr_map_ids,strerror(errno)); return 1; }
    int map = bpf_map_get_fd_by_id(ids[0]);
    if (map < 0) return 1;
    struct bpf_map_info details = {};
    size = sizeof(details);
    __u32 key = 0, value = 0;
    int bad = bpf_map_get_info_by_fd(map, &details, &size) ||
        strcmp(details.name, "rx_config") || details.type != BPF_MAP_TYPE_ARRAY ||
        details.key_size != 4 || details.value_size != 4 || details.max_entries != 1 ||
        !(details.map_flags & BPF_F_RDONLY_PROG) ||
        bpf_map_lookup_elem(map, &key, &value) || value != expected;
    // A map reopened by ID reports EPERM from freeze once frozen because
    // map_get_sys_perms removes write permission. Verify actual writes fail too.
    if (!bad) {
        if (bpf_map_freeze(map) && errno != EPERM && errno != EBUSY) bad = 1;
        if (!bad && (!bpf_map_update_elem(map, &key, &value, BPF_ANY) || errno != EPERM)) bad = 1;
    }
    if (bad) fprintf(stderr,"receive map %s type=%u key=%u value=%u max=%u flags=%u ip=%u expected=%u\n",details.name,details.type,details.key_size,details.value_size,details.max_entries,details.map_flags,value,expected);
    close(map); return bad;
}

static int detach(int cg)
{
    __u32 ids[128], flags[128];
    LIBBPF_OPTS(bpf_prog_query_opts, query,
        .prog_ids = ids, .prog_attach_flags = flags, .prog_cnt = 128);
    if (bpf_prog_query_opts(cg, BPF_LSM_CGROUP, &query)) return -1;
    __u32 count = query.prog_cnt;
    for (__u32 i = 0; i < count; i++) {
        int fd = bpf_prog_get_fd_by_id(ids[i]);
        struct bpf_prog_info info = {};
        __u32 size = sizeof(info);
        if (fd < 0) { perror("get program fd"); return -1; }
        if (bpf_prog_get_info_by_fd(fd, &info, &size)) { close(fd); return -1; }
        if (!strncmp((char *)info.name, "adcap_", 6) && bpf_prog_detach2(fd, cg, BPF_LSM_CGROUP)) {
            perror("detach program"); close(fd); return -1;
        }
        close(fd);
    }
    return 0;
}
static int receive_policy(int argc, char **argv)
{
    int attach = !strcmp(argv[1], "rx-attach");
    int check = !strcmp(argv[1], "rx-check");
    if (((attach || check) && argc != 6) || ((!attach && !check) && argc != 4)) return 2;
    int ns = open(argv[2], O_RDONLY);
    if (ns < 0 || setns(ns, CLONE_NEWNET)) { perror("target network namespace"); return 1; }
    close(ns);
    unsigned int iface = if_nametoindex(argv[3]);
    __u32 id = 0;
    if (!iface || bpf_xdp_query_id(iface, XDP_FLAGS_SKB_MODE, &id)) {
        fprintf(stderr, "cannot query target interface XDP policy\n"); return 1;
    }
    if (!attach && !check) {
        if (!id) return 0;
        int fd = bpf_prog_get_fd_by_id(id);
        struct bpf_prog_info info = {};
        __u32 size = sizeof(info);
        if (fd < 0 || bpf_prog_get_info_by_fd(fd, &info, &size) || strcmp((char *)info.name, "adcap_ingress")) {
            fprintf(stderr, "refusing to detach an unrelated XDP program\n");
            if (fd >= 0) close(fd);
            return 1;
        }
        LIBBPF_OPTS(bpf_xdp_attach_opts, opts, .old_prog_fd = fd);
        int ret = bpf_xdp_detach(iface, XDP_FLAGS_SKB_MODE, &opts);
        close(fd);
        return ret ? 1 : 0;
    }
    struct if_nameindex *interfaces = if_nameindex();
    if (!interfaces) return 1;
    int extra = 0;
    for (struct if_nameindex *i = interfaces; i->if_index; i++)
        if (strcmp(i->if_name, "lo") && strcmp(i->if_name, argv[3])) extra = 1;
    if_freenameindex(interfaces);
    if (extra) { fprintf(stderr, "unexpected extra capture interface\n"); return 1; }
    if (attach && id) { fprintf(stderr, "refusing to replace existing XDP policy\n"); return 1; }
    struct in_addr own_ip;
    if (inet_pton(AF_INET, argv[4], &own_ip) != 1 || !own_ip.s_addr) {
        fprintf(stderr, "expected a nonzero instance IPv4 address\n"); return 1;
    }
    struct bpf_object *obj = load_policy(argv[5], BPF_PROG_TYPE_XDP);
    if (!obj) return 1;
    struct bpf_program *prog;
    if (check) {
        prog = bpf_object__find_program_by_name(obj, "adcap_ingress");
        int fd = id ? bpf_prog_get_fd_by_id(id) : -1;
        int bad = fd < 0 || !prog || !same_program(bpf_program__fd(prog), fd) || check_rx_config(fd, own_ip.s_addr);
        if (fd >= 0) close(fd);
        bpf_object__close(obj);
        if (bad) fprintf(stderr, "receive policy verification failed\n");
        return bad ? 1 : 0;
    }
    struct bpf_map *map = bpf_object__find_map_by_name(obj, "rx_config");
    prog = bpf_object__find_program_by_name(obj, "adcap_ingress");
    __u32 key = 0;
    if (!map || !prog || bpf_map_update_elem(bpf_map__fd(map), &key, &own_ip.s_addr, BPF_ANY) ||
        bpf_map_freeze(bpf_map__fd(map))) {
        fprintf(stderr, "receive configuration failed\n"); bpf_object__close(obj); return 1;
    }
    int ret = bpf_xdp_attach(iface, bpf_program__fd(prog),
        XDP_FLAGS_SKB_MODE | XDP_FLAGS_UPDATE_IF_NOEXIST, NULL);
    bpf_object__close(obj);
    if (ret) { fprintf(stderr, "receive guard attach failed: %d\n", ret); return 1; }
    puts("instance receive boundary attached"); return 0;
}

int main(int argc, char **argv)
{
    if (argc >= 2 && (!strcmp(argv[1], "rx-attach") || !strcmp(argv[1], "rx-detach") || !strcmp(argv[1], "rx-check")))
        return receive_policy(argc, argv);
    if (argc < 3 || (strcmp(argv[1], "attach") && strcmp(argv[1], "detach") && strcmp(argv[1], "check"))) {
        fprintf(stderr, "usage: policy attach CGROUP OBJECT | policy detach CGROUP\n");
        return 2;
    }
    int cg = open(argv[2], O_RDONLY | O_DIRECTORY);
    if (cg < 0) { perror("open cgroup"); return 1; }
    struct statfs fs;
    struct stat target, root;
    if (fstatfs(cg, &fs) || fs.f_type != 0x63677270 || fstat(cg, &target) ||
        stat("/sys/fs/cgroup", &root) ||
        (target.st_dev == root.st_dev && target.st_ino == root.st_ino)) {
        fprintf(stderr, "target must be a dedicated cgroup v2 child, never the host root\n");
        close(cg); return 1;
    }
    if (!strcmp(argv[1], "detach")) {
        int ret = detach(cg); if (ret) perror("detach"); close(cg); return ret ? 1 : 0;
    }
    if (argc != 4) return 2;
    if (!strcmp(argv[1], "check")) {
        struct bpf_object *expected = load_policy(argv[3], BPF_PROG_TYPE_LSM);
        if (!expected) return 1;
        int ret = check_cgroup(cg, expected);
        bpf_object__close(expected);
        if (ret) fprintf(stderr, "send policy verification failed\n");
        return ret;
    }
    LIBBPF_OPTS(bpf_prog_query_opts, query);
    if (bpf_prog_query_opts(cg, BPF_LSM_CGROUP, &query) || query.prog_cnt) {
        fprintf(stderr, "refusing to replace existing cgroup LSM policies\n");
        close(cg); return 1;
    }
    struct bpf_object *obj = bpf_object__open_file(argv[3], NULL);
    if (libbpf_get_error(obj)) { fprintf(stderr, "open BPF object failed\n"); return 1; }
    struct bpf_program *candidate;
    bpf_object__for_each_program(candidate, obj)
        bpf_program__set_autoload(candidate, bpf_program__type(candidate) == BPF_PROG_TYPE_LSM);
    if (bpf_object__load(obj)) { fprintf(stderr, "load BPF object failed\n"); return 1; }
    struct bpf_program *prog;
    bpf_object__for_each_program(prog, obj) {
        if (bpf_program__type(prog) != BPF_PROG_TYPE_LSM) continue;
        if (bpf_prog_attach(bpf_program__fd(prog), cg, BPF_LSM_CGROUP, 0)) {
            perror("attach");
            // Only use this loader on a dedicated, empty challenge cgroup.
            detach(cg); bpf_object__close(obj); close(cg); return 1;
        }
    }
    bpf_object__close(obj); close(cg);
    puts("receive-only packet policy attached"); return 0;
}

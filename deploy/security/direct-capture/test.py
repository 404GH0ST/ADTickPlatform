#!/usr/bin/env python3
"""Isolated direct-tcpdump proof: temporarily guards only its disposable cgroup.

Requires Docker, cgroup v2, active BPF LSM, clang, gcc, ldd, and libbpf headers.
The one-shot loader needs elevated BPF privileges; challenge containers do not.
"""
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import time
from urllib.request import urlopen
import uuid

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]


def run(*args, **kwargs):
    result = subprocess.run(args, text=True, capture_output=True, **kwargs)
    if result.returncode:
        raise RuntimeError(f"{' '.join(args[:4])}: {result.stderr}{result.stdout}")
    return result.stdout.strip()


def capture_start(name, interface, path):
    errors = tempfile.TemporaryFile()
    process = subprocess.Popen([
        'docker', 'exec', name, 'tcpdump', '-Z', 'root', '-i', interface,
        '-nn', '-s', '0', '-U', '-w', path,
    ], stdout=subprocess.DEVNULL, stderr=errors)
    for _ in range(100):
        if b'listening on' in os.pread(errors.fileno(), 8192, 0):
            return process, errors
        if process.poll() is not None:
            raise RuntimeError(os.pread(errors.fileno(),8192,0).decode())
        time.sleep(0.05)
    process.kill()
    raise RuntimeError('tcpdump did not start')


def capture_stop(name, capture, path):
    process, errors = capture
    try:
        run('docker','exec',name,'python3','-c', """
import os,signal
for entry in os.listdir('/proc'):
    if entry.isdigit():
        try:
            if open('/proc/'+entry+'/comm').read().strip()=='tcpdump':
                os.kill(int(entry),signal.SIGINT)
        except (OSError,ProcessLookupError): pass
""")
        assert process.wait(timeout=5)==0, os.pread(errors.fileno(),8192,0).decode()
        data=run('docker','exec',name,'python3','-c',f"print(open({path!r},'rb').read().hex())")
        return bytes.fromhex(data)
    finally:
        errors.close()
        if process.poll() is None:
            process.kill()
            process.wait()


def cross_team_receive_test(name, peer, checker, own_ip, peer_ip, manager):
    markers=['FOREIGN_BROADCAST_FLAG','FOREIGN_MULTICAST_FLAG','FOREIGN_FLOODED_FLAG',
             'FOREIGN_ARP_FLAG','FOREIGN_IPV6_FLAG','FOREIGN_VLAN_FLAG','FOREIGN_CHECKER_FLAG']
    own_marker='OWN_CHECKER_FLAG_VISIBLE'
    # Only the trusted fixture sender has unrestricted NET_RAW, in a throwaway network.
    emit = r'''
import socket,struct,json
from urllib.request import Request,urlopen
own_ip,peer_ip=OWN_IP,PEER_IP
source_mac=bytes.fromhex(open('/sys/class/net/eth0/address').read().strip().replace(':',''))
raw=socket.socket(socket.AF_PACKET,socket.SOCK_RAW);raw.bind(('eth0',0))
def ipv4(destination,payload):
    udp=struct.pack('!HHHH',19000,19001,8+len(payload),0)+payload
    header=struct.pack('!BBHHHBBH4s4s',0x45,0,20+len(udp),0,0,64,17,0,socket.inet_aton(peer_ip),socket.inet_aton(destination))
    words=struct.unpack('!10H',header);checksum=sum(words)
    checksum=(checksum&65535)+(checksum>>16);checksum=(checksum&65535)+(checksum>>16)
    header=header[:10]+struct.pack('!H',~checksum&65535)+header[12:]
    return header+udp
def send(mac,ethertype,data):raw.send(mac+source_mac+struct.pack('!H',ethertype)+data)
broadcast=b'\xff'*6
for _ in range(3):
    send(broadcast,0x800,ipv4('255.255.255.255',b'FOREIGN_BROADCAST_FLAG'))
    send(bytes.fromhex('01005e010203'),0x800,ipv4('224.1.2.3',b'FOREIGN_MULTICAST_FLAG'))
    send(broadcast,0x800,ipv4(peer_ip,b'FOREIGN_FLOODED_FLAG'))
    arp=struct.pack('!HHBBH',1,0x800,6,4,1)+source_mac+socket.inet_aton(peer_ip)+b'\0'*6+socket.inet_aton(peer_ip)
    send(broadcast,0x806,arp+b'FOREIGN_ARP_FLAG')
    send(broadcast,0x86dd,b'\x60'+b'\0'*39+b'FOREIGN_IPV6_FLAG')
    send(broadcast,0x8100,struct.pack('!HH',1,0x800)+ipv4(peer_ip,b'FOREIGN_VLAN_FLAG'))
'''.replace('OWN_IP',repr(own_ip)).replace('PEER_IP',repr(peer_ip))
    checker_write = """
import json
from urllib.request import Request,urlopen
for host,flag in ((PEER_IP,'FOREIGN_CHECKER_FLAG'),(OWN_IP,'OWN_CHECKER_FLAG_VISIBLE')):
    request=Request('http://'+host+':8080/flag',data=json.dumps({'flag':flag}).encode(),headers={'Content-Type':'application/json','X-Checker-Token':'sample-http-checker-token'})
    with urlopen(request,timeout=5) as response:assert response.status==200
""".replace('OWN_IP',repr(own_ip)).replace('PEER_IP',repr(peer_ip))
    baseline=capture_start(name,'eth0','/tmp/baseline.pcap')
    try:
        run('docker','exec',peer,'python3','-c',emit)
        run('docker','exec',checker,'python3','-c',checker_write)
        time.sleep(1.3)
    finally:
        data=capture_stop(name,baseline,'/tmp/baseline.pcap')
    print('Baseline capture bytes:',len(data),flush=True)
    assert b'FOREIGN_BROADCAST_FLAG' in data, 'baseline did not demonstrate shared bridge visibility'
    assert b'FOREIGN_FLOODED_FLAG' in data, 'baseline did not receive flooded foreign IPv4'
    for marker in markers[:-1]:
        assert marker.encode() in data, f'baseline did not exercise {marker}'
    assert b'FOREIGN_CHECKER_FLAG' not in data, 'ordinary peer checker unicast should not reach this interface'

    print('Baseline confirmed: shared-bridge broadcasts and flooded foreign frames are capturable',flush=True)
    print(run(*manager,'rx-attach','/target.netns','eth0',own_ip,'/work/capture.bpf.o'),flush=True)
    run(*manager,'rx-check','/target.netns','eth0',own_ip,'/work/capture.bpf.o')
    assert subprocess.run([*manager,'rx-check','/target.netns','eth0',peer_ip,'/work/capture.bpf.o'],capture_output=True).returncode != 0
    try:
        for interface in ('eth0','any'):
            path='/tmp/guarded-'+interface+'.pcap'
            capture=capture_start(name,interface,path)
            try:
                run('docker','exec',peer,'python3','-c',emit)
                run('docker','exec',checker,'python3','-c',checker_write)
                time.sleep(1.3)
            finally:
                data=capture_stop(name,capture,path)
            assert own_marker.encode() in data, 'own checker update was not visible'
            for marker in markers:
                assert marker.encode() not in data, f'{interface} leaked {marker}'
            print(f'{interface}: own checker flag visible; foreign checker/broadcast/multicast/flooded/ARP/IPv6/VLAN payloads absent',flush=True)
        # A participant root process cannot disable the host-owned ingress policy.
        denied=subprocess.run(['docker','exec',name,'python3','-c',"""
import ctypes,errno,socket,struct
libc=ctypes.CDLL(None,use_errno=True)
assert libc.setns(-1,0)==-1
assert ctypes.get_errno()==errno.EPERM
# Request a real XDP detach through rtnetlink, as container root.
netlink=socket.socket(socket.AF_NETLINK,socket.SOCK_RAW,socket.NETLINK_ROUTE)
netlink.bind((0,0))
netlink.settimeout(3)
info=struct.pack('=BBHiII',0,0,0,socket.if_nametoindex('eth0'),0,0)
attributes=struct.pack('=HHi',8,1,-1)+struct.pack('=HHI',8,3,2)
nested=struct.pack('=HH',len(attributes)+4,43|0x8000)+attributes
request=struct.pack('=IHHII',16+len(info)+len(nested),16,5,1,0)+info+nested
netlink.send(request)
reply=netlink.recv(4096)
assert struct.unpack_from('=H',reply,4)[0]==2
assert struct.unpack_from('=i',reply,16)[0]==-errno.EPERM, reply.hex()
print('container-root XDP detach: denied')
"""],capture_output=True,text=True)
        assert denied.returncode==0, denied.stderr
        print(denied.stdout.strip(),flush=True)
    finally:
        run(*manager,'rx-detach','/target.netns','eth0')
    print('Receive boundary detached cleanly',flush=True)


def main():
    if 'bpf' not in Path('/sys/kernel/security/lsm').read_text().strip().split(','):
        raise RuntimeError('BPF LSM must be active; no unguarded fallback is permitted')
    suffix = uuid.uuid4().hex[:12]
    name = f'adcap-test-{suffix}'
    image = f'adplatform/direct-capture-test:{suffix}'
    network = f'adcap-net-{suffix}'
    peer = f'adcap-peer-{suffix}'
    checker = f'adcap-checker-{suffix}'
    with tempfile.TemporaryDirectory(prefix='adcap-test-') as folder:
        work = Path(folder)
        work.chmod(0o755)
        run('clang', '-O2', '-g', '-target', 'bpf', '-c', str(HERE / 'capture.bpf.c'), '-o', str(work / 'capture.bpf.o'))
        run('gcc', '-O2', '-Wall', '-Wextra', str(HERE / 'policy.c'), '-o', str(work / 'policy'), '-lbpf')
        interpreter = None
        for line in run('ldd', str(work / 'policy')).splitlines():
            paths = re.findall(r'/[^\s()]+', line)
            if paths:
                dependency = Path(paths[-1])
                shutil.copy(dependency, work / dependency.name)
                if 'ld-linux' in dependency.name:
                    interpreter = dependency.name
        if not interpreter:
            raise RuntimeError('could not identify the Linux dynamic linker')
        manager = [
            'docker', 'run', '--rm', '--cap-drop', 'ALL', '--cap-add', 'BPF',
            '--cap-add', 'PERFMON', '--cap-add', 'NET_ADMIN', '--cap-add', 'SYS_ADMIN',
            '--security-opt', 'no-new-privileges:true', '--cgroupns', 'host',
            '-v', '/sys/fs/cgroup:/sys/fs/cgroup:rw', '-v', f'{work}:/work:ro',
            'debian:trixie-slim', f'/work/{interpreter}', '--library-path', '/work', '/work/policy',
        ]
        attached = False
        cgroup = None
        try:
            print('Building disposable example image', flush=True)
            run('docker', 'build', '-t', image, str(ROOT / 'examples/sample-http-challenge/service'))
            run('docker','network','create',network)
            # Start inert. Load the policy BEFORE running any challenge code.
            run('docker', 'run', '--rm', '-d', '--name', name, '--cap-drop', 'ALL',
                '--cap-add', 'NET_RAW', '--cap-add', 'CHOWN', '--cap-add', 'SETUID', '--cap-add', 'SETGID',
                '--security-opt', 'no-new-privileges:true', '-p', '127.0.0.1::8080',
                '--network',network,'--entrypoint', 'sleep', image, 'infinity')
            pid = run('docker', 'inspect', '-f', '{{.State.Pid}}', name)
            relative = Path(f'/proc/{pid}/cgroup').read_text().strip().split('::')[-1]
            cgroup = '/sys/fs/cgroup' + relative
            offset=manager.index('debian:trixie-slim')
            manager[offset:offset]=['-v',f'/proc/{pid}/ns/net:/target.netns:ro']
            own_ip=run('docker','inspect','-f',f'{{{{(index .NetworkSettings.Networks "{network}").IPAddress}}}}',name)
            run('docker','run','--rm','-d','--name',peer,'--cap-drop','ALL','--cap-add','NET_RAW',
                '--security-opt','no-new-privileges:true','--network',network,
                '--entrypoint','python3',image,'/app/service.py')
            peer_ip=run('docker','inspect','-f',f'{{{{(index .NetworkSettings.Networks "{network}").IPAddress}}}}',peer)
            run('docker','run','--rm','-d','--name',checker,'--cap-drop','ALL',
                '--security-opt','no-new-privileges:true','--network',network,
                '--entrypoint','sleep',image,'infinity')

            if relative == '/' or suffix not in name:
                raise RuntimeError('refusing to attach to the host cgroup root')
            print(run(*manager, 'attach', cgroup, '/work/capture.bpf.o'), flush=True)
            attached = True
            run(*manager, 'check', cgroup, '/work/capture.bpf.o')
            run('docker', 'exec', '-d', name, 'python3', '/app/service.py')
            port = int(run('docker', 'port', name, '8080/tcp').rsplit(':', 1)[1])
            url = f'http://127.0.0.1:{port}/health'
            for attempt in range(40):
                try:
                    with urlopen(url, timeout=2) as response:
                        assert response.status == 200
                    break
                except OSError:
                    if attempt == 39:
                        raise
                    time.sleep(0.1)
            cross_team_receive_test(name,peer,checker,own_ip,peer_ip,manager)
            capture = subprocess.Popen([
                'docker', 'exec', name, 'tcpdump', '-p', '-Q', 'in', '-i', 'eth0',
                '-nn', '-U', '-c', '2', '-w', '/tmp/incoming.pcap', 'tcp', 'dst', 'port', '8080',
            ], text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            try:
                for _ in range(50):
                    with urlopen(url, timeout=2) as response:
                        json.load(response)
                    if capture.poll() is not None:
                        break
                    time.sleep(0.1)
                output, errors = capture.communicate(timeout=5)
                assert capture.returncode == 0, output + errors
                packets = run('docker', 'exec', name, 'tcpdump', '-nn', '-r', '/tmp/incoming.pcap')
                assert len(packets.splitlines()) == 2 and all('.8080:' in line for line in packets.splitlines()), packets
                print('Ordinary tcpdump captured incoming service traffic; HTTP remains healthy', flush=True)
            finally:
                if capture.poll() is None:
                    capture.kill()
                    capture.communicate()
            test = r'''
import ctypes, errno, os, socket, struct
s = socket.socket(socket.AF_PACKET, socket.SOCK_RAW, socket.htons(3))
s.bind(('eth0', 0))
payload = b'\0' * 60
for name, operation in (
    ('send', lambda: s.send(payload)),
    ('sendto', lambda: s.sendto(payload, ('eth0', 0))),
    ('sendmsg', lambda: s.sendmsg([payload])),
    ('write', lambda: os.write(s.fileno(), payload)),
    ('writev', lambda: os.writev(s.fileno(), [payload])),
    ('TX_RING', lambda: s.setsockopt(263, 13, struct.pack('IIII',4096,1,2048,2))),
):
    try:
        operation()
    except PermissionError:
        print(name + ': denied')
    else:
        raise RuntimeError(name + ': injection path allowed')
# sendmmsg has a separate userspace syscall entry; it must also be denied.
class IOVec(ctypes.Structure):
    _fields_ = [('base',ctypes.c_void_p),('length',ctypes.c_size_t)]
class MsgHdr(ctypes.Structure):
    _fields_ = [('name',ctypes.c_void_p),('namelen',ctypes.c_uint),
                ('iov',ctypes.POINTER(IOVec)),('iovlen',ctypes.c_size_t),
                ('control',ctypes.c_void_p),('controllen',ctypes.c_size_t),('flags',ctypes.c_int)]
class MMsgHdr(ctypes.Structure):
    _fields_ = [('header',MsgHdr),('length',ctypes.c_uint)]
buffer = ctypes.create_string_buffer(payload)
iov = IOVec(ctypes.cast(buffer,ctypes.c_void_p),len(payload))
message = MMsgHdr(MsgHdr(None,0,ctypes.pointer(iov),1,None,0,0),0)
libc = ctypes.CDLL(None,use_errno=True)
assert libc.sendmmsg(s.fileno(),ctypes.byref(message),1,0) == -1
assert ctypes.get_errno() == errno.EPERM, ctypes.get_errno()
print('sendmmsg: denied')
for family in (socket.AF_INET, socket.AF_INET6):
    try:
        socket.socket(family, socket.SOCK_RAW, socket.IPPROTO_RAW)
    except PermissionError:
        print('raw IP socket: denied')
    else:
        raise RuntimeError('raw IP creation allowed')
for level, option, family in ((0,19,socket.AF_INET),(41,75,socket.AF_INET6)):
    stream = socket.socket(family,socket.SOCK_STREAM)
    try:
        stream.setsockopt(level,option,1)
    except PermissionError:
        print('transparent source binding: denied')
    else:
        raise RuntimeError('transparent binding allowed')
'''
            print(run('docker', 'exec', name, 'python3', '-c', test), flush=True)
            with urlopen(url, timeout=2) as response:
                assert response.status == 200
            run(*manager, 'detach', cgroup)
            attached = False
            print('Policy detached and test resources cleaned up', flush=True)
        finally:
            if attached:
                subprocess.run([*manager, 'detach', cgroup], capture_output=True)
            subprocess.run(['docker', 'rm', '-f', checker, peer, name], capture_output=True)
            subprocess.run(['docker','network','rm',network],capture_output=True)
            subprocess.run(['docker', 'image', 'rm', image], capture_output=True)


if __name__ == '__main__':
    main()

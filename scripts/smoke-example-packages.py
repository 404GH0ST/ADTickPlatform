#!/usr/bin/env python3
"""Check example images in disposable containers without changing any platform stack."""
import json
import os
from pathlib import Path
import socket
import subprocess
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from threading import Thread
import time
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode
from urllib.request import Request, urlopen
import uuid

ROOT = Path(__file__).resolve().parent.parent
RUN_ID = uuid.uuid4().hex[:12]


def run(*args, **kwargs):
    return subprocess.run(args, text=True, capture_output=True, **kwargs)


def docker(*args, **kwargs):
    result = run("docker", *args, **kwargs)
    if result.returncode:
        raise RuntimeError(f"docker {' '.join(args[:3])}: {result.stderr or result.stdout}")
    return result.stdout.strip()


def request(url, payload=None, headers=None):
    data = None if payload is None else json.dumps(payload).encode()
    try:
        with urlopen(Request(url, data=data, headers={"Content-Type": "application/json", **(headers or {})}), timeout=5) as response:
            return response.status, json.load(response)
    except HTTPError as exc:
        return exc.code, json.load(exc)


def wait_ready(url):
    for _ in range(60):
        try:
            if request(url)[0] == 200:
                return
        except (URLError, OSError):
            pass
        time.sleep(0.25)
    raise RuntimeError(f"service did not become ready: {url}")


def exercise_exploit(path, port, flag):
    """Use the real exploit and a strict API fixture to check the wire contract.

    Authoritative flag validation and SSH grants are covered by game-core tests.
    This fixture catches stale /unlock calls, bearer token parsing, verdict
    parsing, configured flag prefixes, and duplicate handling in the examples.
    """
    attempts = []

    class PlatformFixture(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def do_POST(self):
            payload = json.loads(self.rfile.read(int(self.headers.get("Content-Length", "0"))))
            status = 200
            if self.path == "/api/v2/authenticate":
                response = {"token": "example-test-token", "token_type": "Bearer"}
            elif self.path == "/api/v2/submit" and self.headers.get("Authorization") == "Bearer example-test-token":
                assert payload == {"flags": [flag]}, payload
                verdict = "duplicate" if attempts else "accepted"
                attempts.append(flag)
                response = {"results": [{"flag": flag, "status": verdict, "challenge_id": 7}], "accepted_count": int(verdict == "accepted")}
            else:
                status, response = 400, {"detail": "unexpected request"}
            encoded = json.dumps(response).encode()
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(encoded)))
            self.end_headers()
            self.wfile.write(encoded)

    server = ThreadingHTTPServer(("127.0.0.1", 0), PlatformFixture)
    thread = Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        env = {**os.environ, "PLATFORM_URL": f"http://127.0.0.1:{server.server_port}", "EMAIL": "example@test.local", "PASSWORD": "local-fixture"}
        accepted = run(sys.executable, str(path), "127.0.0.1", str(port), "7", env=env, timeout=20)
        assert accepted.returncode == 0, accepted.stdout + accepted.stderr
        assert "SSH automatically unlocked" in accepted.stdout, accepted.stdout
        duplicate = run(sys.executable, str(path), "127.0.0.1", str(port), "7", env=env, timeout=20)
        assert duplicate.returncode != 0 and "No new capture accepted" in duplicate.stdout, duplicate.stdout
        assert len(attempts) == 2, attempts
    finally:
        server.shutdown()
        server.server_close()
        thread.join()


def smoke(kind):
    package = ROOT / "examples" / f"sample-{kind}-challenge"
    name = f"example-smoke-{kind}-{RUN_ID}"
    service_image = f"adplatform/example-smoke-{kind}:service-{RUN_ID}"
    checker_image = f"adplatform/example-smoke-{kind}:checker-{RUN_ID}"
    token = uuid.uuid4().hex
    images = []
    try:
        for image, context in ((service_image, "service"), (checker_image, "checker")):
            print(f"{kind}: building {context}", flush=True)
            docker("build", "-t", image, str(package / context))
            images.append(image)
        capabilities = ("CHOWN", "DAC_OVERRIDE", "FOWNER", "SETGID", "SETUID",
                        "NET_BIND_SERVICE", "SYS_CHROOT", "AUDIT_WRITE")
        security = ["--cap-drop", "ALL", "--security-opt", "no-new-privileges:true"]
        for capability in capabilities:
            security.extend(("--cap-add", capability))
        docker("run", "--rm", "-d", *security, "--name", name,
               "-p", "127.0.0.1::8080", "-p", "127.0.0.1::22",
               "-e", "PORT=8080", "-e", f"AD_CHECKER_TOKEN={token}", service_image)
        port = int(docker("port", name, "8080/tcp").rsplit(":", 1)[1])
        ssh_port = int(docker("port", name, "22/tcp").rsplit(":", 1)[1])
        base = f"http://127.0.0.1:{port}"
        health = "/health" if kind == "http" else "/v1/health"
        wait_ready(base + health)
        assert " L " in docker("exec", name, "passwd", "-S", "root"), "root should start locked"
        assert "tcpdump version" in docker("exec", name, "tcpdump", "--version")
        # Installing tcpdump must not give even challenge root raw sockets.
        capture = run("docker", "exec", name, "tcpdump", "-p", "-i", "any",
                      "-nn", "-c", "1", timeout=5)
        assert capture.returncode != 0 and "permitted" in capture.stderr.lower(), capture.stderr
        docker("exec", name, "python3", "-c", """
import socket
try:
    sock = socket.socket(socket.AF_PACKET, socket.SOCK_RAW, socket.htons(3))
except PermissionError:
    pass
else:
    sock.close()
    raise SystemExit('challenge root unexpectedly has raw socket access')
""")
        with socket.create_connection(("127.0.0.1", ssh_port), timeout=5) as ssh:
            assert ssh.recv(256).startswith(b"SSH-"), "SSH daemon is not running"
        assert request(base + "/unlock")[0] == 404, "legacy proof endpoint remains exposed"

        def checker(phase, flag="", metadata="", success=True, expected_code=None):
            result = run("docker", "run", "--rm", "--network", f"container:{name}",
                         "-e", "AD_TARGET_HOST=127.0.0.1", "-e", "AD_TARGET_PORT=8080",
                         "-e", f"AD_CHECKER_TOKEN={token}", "-e", f"AD_FLAG={flag}",
                         "-e", f"AD_METADATA={metadata}", checker_image, phase, timeout=30)
            assert (result.returncode == 0) == success, f"{kind} {phase}: {result.stdout} {result.stderr}"
            if expected_code is not None:
                assert result.returncode == expected_code, result.stderr
            return result.stdout.strip()

        assert 'ADPLATFORM_CHECKER_CAPABILITIES={"service_state":true}' in checker("validate")
        assert "ADPLATFORM_SERVICE_STATE=" in checker("check")
        for tick in (1, 2):
            flag = f"CUSTOM{{example-{kind}-tick-{tick}}}"
            metadata = checker("put", flag)
            checker("get", flag, metadata)
            if tick == 2:
                checker("get", f"CUSTOM{{example-{kind}-tick-1}}", metadata, success=False, expected_code=42)
            if kind == "http":
                assert request(base + "/leak")[1]["flag"] == flag
            elif kind == "lfi":
                leak = "/v1/render?" + urlencode({"template": "../../state/flag.txt"})
                assert request(base + leak)[1]["content"].strip() == flag
            else:
                leak = "/v1/diagnostics?" + urlencode({"host": "localhost; cat /opt/ad/exposed/flag.txt"})
                assert flag in request(base + leak)[1]["result"]["stdout"]
            if kind != "http":
                exercise_exploit(package / "exploit.py", port, flag)
        assert "ADPLATFORM_SERVICE_STATE=" in checker("check")
        route = "/flag" if kind == "http" else "/v1/flag"
        assert request(base + route, {"flag": "unauthorized"})[0] == 403
        checker("get", flag, metadata)
        # Possession of the instance checker token can poison storage, but the
        # checker must report a definite retention failure instead of accepting it.
        assert request(base + route, {"flag": "DECOY{replaced}"}, {"X-Checker-Token": token})[0] == 200
        checker("get", flag, metadata, success=False, expected_code=42)
        metadata = checker("put", flag)
        checker("get", flag, metadata)
        # The same credential setter used by the controller must unlock root.
        docker("exec", "-i", name, "chpasswd", input=f"root:Test-{token}!\n")
        assert " P " in docker("exec", name, "passwd", "-S", "root")
        docker("restart", name)
        port = int(docker("port", name, "8080/tcp").rsplit(":", 1)[1])
        base = f"http://127.0.0.1:{port}"
        wait_ready(base + health)
        assert " L " in docker("exec", name, "passwd", "-S", "root")
        checker("get", flag, metadata)
        print(f"{kind}: passed checker cycles, flag capture, tcpdump privilege boundary, SSH contract, and restart persistence", flush=True)
    except Exception:
        print(run("docker", "logs", name).stdout, file=sys.stderr)
        print(run("docker", "logs", name).stderr, file=sys.stderr)
        raise
    finally:
        run("docker", "rm", "-f", name)
        for image in images:
            run("docker", "image", "rm", image)


if __name__ == "__main__":
    for example in ("http", "lfi", "rce"):
        smoke(example)
    print("All example challenge packages passed.")

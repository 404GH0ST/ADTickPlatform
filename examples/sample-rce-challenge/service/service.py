#!/usr/bin/env python3
import json
import os
import pwd
import subprocess
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import parse_qs, urlparse


STATE_DIR = Path(os.environ.get("AD_STATE_DIR", "/opt/ad/state"))
PUBLIC_DIR = Path(os.environ.get("AD_PUBLIC_DIR", "/opt/ad/public"))
EXPOSED_DIR = Path(os.environ.get("AD_EXPOSED_DIR", "/opt/ad/exposed"))
SECRET_DIR = STATE_DIR / "secret"
FLAG_PATH = STATE_DIR / "flag.txt"
EXPOSED_FLAG_PATH = EXPOSED_DIR / "flag.txt"
EXPOSED_UNLOCK_PROOF_PATH = EXPOSED_DIR / "unlock-proof.txt"
SERVICE_PORT = int(os.environ.get("PORT", os.environ.get("AD_PLATFORM_SERVICE_PORT", "8080")))
UNLOCK_PROOF = os.environ.get("AD_PLATFORM_UNLOCK_PROOF", "missing-unlock-proof")
CHECKER_TOKEN = os.environ.get("AD_CHECKER_TOKEN", "sample-rce-checker-token")


def bootstrap() -> None:
    STATE_DIR.mkdir(parents=True, exist_ok=True)
    PUBLIC_DIR.mkdir(parents=True, exist_ok=True)
    EXPOSED_DIR.mkdir(parents=True, exist_ok=True)
    SECRET_DIR.mkdir(parents=True, exist_ok=True)
    (PUBLIC_DIR / "status.txt").write_text("sample RCE diagnostics online\n", encoding="utf-8")
    (PUBLIC_DIR / "status.txt").chmod(0o644)
    (SECRET_DIR / "unlock-proof.txt").write_text(f"{UNLOCK_PROOF}\n", encoding="utf-8")
    (SECRET_DIR / "unlock-proof.txt").chmod(0o600)
    publish_exposed(EXPOSED_UNLOCK_PROOF_PATH, UNLOCK_PROOF)


def read_flag() -> str:
    try:
        return FLAG_PATH.read_text(encoding="utf-8").strip()
    except OSError:
        return ""


def write_flag(flag: str) -> None:
    STATE_DIR.mkdir(parents=True, exist_ok=True)
    tmp_path = FLAG_PATH.with_suffix(".tmp")
    tmp_path.write_text(f"{flag}\n", encoding="utf-8")
    tmp_path.chmod(0o600)
    os.replace(tmp_path, FLAG_PATH)
    publish_exposed(EXPOSED_FLAG_PATH, flag)


def publish_exposed(path: Path, value: str) -> None:
    EXPOSED_DIR.mkdir(parents=True, exist_ok=True)
    EXPOSED_DIR.chmod(0o755)
    tmp_path = path.with_suffix(".tmp")
    tmp_path.write_text(f"{value}\n", encoding="utf-8")
    tmp_path.chmod(0o444)
    os.replace(tmp_path, path)


def sandbox_user() -> tuple[int, int] | None:
    if os.geteuid() != 0:
        return None
    try:
        user = pwd.getpwnam("nobody")
    except KeyError:
        return None
    return user.pw_uid, user.pw_gid


def drop_to_sandbox(uid: int, gid: int):
    def inner() -> None:
        os.setgroups([])
        os.setgid(gid)
        os.setuid(uid)

    return inner


def run_diagnostics(host: str) -> dict[str, str | int]:
    # Intentionally vulnerable command injection for the RCE sample challenge.
    # A safe implementation would pass arguments as a list and validate `host`.
    command = f"printf 'diagnostic for '; echo {host}"
    user = sandbox_user()
    result = subprocess.run(
        command,
        shell=True,
        cwd=str(PUBLIC_DIR),
        capture_output=True,
        text=True,
        timeout=3,
        check=False,
        preexec_fn=drop_to_sandbox(*user) if user is not None else None,
    )
    return {
        "command": command,
        "exit_code": result.returncode,
        "stdout": result.stdout,
        "stderr": result.stderr,
    }


class RCESampleHandler(BaseHTTPRequestHandler):
    server_version = "ADPlatformRCESample/1.0"

    def log_message(self, fmt: str, *args) -> None:
        return

    def _send_json(self, status: int, payload: dict) -> None:
        body = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self) -> None:
        parsed = urlparse(self.path)

        if parsed.path == "/v1/health":
            self._send_json(HTTPStatus.OK, {"status": "ok"})
            return

        if parsed.path == "/v1/status":
            try:
                status = (PUBLIC_DIR / "status.txt").read_text(encoding="utf-8").strip()
            except OSError:
                self._send_json(HTTPStatus.NOT_FOUND, {"status": "missing"})
                return
            self._send_json(HTTPStatus.OK, {"status": "ok", "message": status})
            return

        if parsed.path == "/v1/flag":
            supplied = parse_qs(parsed.query).get("value", [""])[0]
            flag = read_flag()
            if supplied and supplied == flag:
                self._send_json(HTTPStatus.OK, {"status": "stored", "flag": supplied})
                return
            self._send_json(HTTPStatus.NOT_FOUND, {"status": "missing"})
            return

        if parsed.path == "/v1/diagnostics":
            host = parse_qs(parsed.query).get("host", ["localhost"])[0]
            try:
                payload = run_diagnostics(host)
            except subprocess.TimeoutExpired:
                self._send_json(HTTPStatus.REQUEST_TIMEOUT, {"status": "timeout"})
                return
            self._send_json(HTTPStatus.OK, {"status": "ok", "result": payload})
            return

        self._send_json(HTTPStatus.NOT_FOUND, {"status": "not_found"})

    def do_POST(self) -> None:
        parsed = urlparse(self.path)
        if parsed.path != "/v1/flag":
            self._send_json(HTTPStatus.NOT_FOUND, {"status": "not_found"})
            return
        if self.headers.get("X-Checker-Token", "") != CHECKER_TOKEN:
            self._send_json(HTTPStatus.FORBIDDEN, {"status": "forbidden"})
            return

        content_length = int(self.headers.get("Content-Length", "0"))
        raw_body = self.rfile.read(content_length)
        try:
            payload = json.loads(raw_body.decode("utf-8") or "{}")
        except json.JSONDecodeError:
            self._send_json(HTTPStatus.BAD_REQUEST, {"status": "invalid_json"})
            return

        flag = payload.get("flag", "")
        if not isinstance(flag, str) or flag == "":
            self._send_json(HTTPStatus.BAD_REQUEST, {"status": "missing_flag"})
            return

        write_flag(flag)
        self._send_json(HTTPStatus.OK, {"status": "stored", "path": "flag.txt"})


def main() -> None:
    bootstrap()
    server = ThreadingHTTPServer(("0.0.0.0", SERVICE_PORT), RCESampleHandler)
    server.serve_forever()


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
import json
import os
import re
import secrets
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import parse_qs, urlparse


STATE_DIR = Path(os.environ.get("AD_STATE_DIR", "/opt/ad/state"))
MESSAGES_DIR = STATE_DIR / "messages"
SECRET_DIR = STATE_DIR / "secret"
FLAG_PATH = STATE_DIR / "flag.txt"
TEMPLATES_DIR = Path(os.environ.get("AD_TEMPLATES_DIR", "/opt/ad/content/templates"))
SERVICE_PORT = int(os.environ.get("PORT", os.environ.get("AD_PLATFORM_SERVICE_PORT", "8080")))
UNLOCK_PROOF = os.environ.get("AD_PLATFORM_UNLOCK_PROOF", "missing-unlock-proof")
CHECKER_TOKEN = os.environ.get("AD_CHECKER_TOKEN", "sample-lfi-checker-token")
TITLE_RE = re.compile(r"^[A-Za-z0-9 _.\-]{1,64}$")


def bootstrap() -> None:
    MESSAGES_DIR.mkdir(parents=True, exist_ok=True)
    SECRET_DIR.mkdir(parents=True, exist_ok=True)
    TEMPLATES_DIR.mkdir(parents=True, exist_ok=True)
    welcome_path = TEMPLATES_DIR / "welcome.txt"
    if not welcome_path.exists():
        welcome_path.write_text("Welcome to sample LFI challenge.\n", encoding="utf-8")
    (SECRET_DIR / "unlock-proof.txt").write_text(f"{UNLOCK_PROOF}\n", encoding="utf-8")


def slot_path(slot: int) -> Path:
    return MESSAGES_DIR / f"slot-{slot}.json"


def read_slot(slot: int) -> dict | None:
    path = slot_path(slot)
    if not path.exists():
        return None
    try:
        with path.open("r", encoding="utf-8") as handle:
            payload = json.load(handle)
    except (json.JSONDecodeError, OSError):
        return None
    if not isinstance(payload, dict):
        return None
    return payload


def write_slot(slot: int, title: str, body: str) -> dict:
    token = secrets.token_hex(16)
    payload = {"slot": slot, "title": title, "body": body, "token": token}
    with slot_path(slot).open("w", encoding="utf-8") as handle:
        json.dump(payload, handle)
    return payload


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


class LFISampleHandler(BaseHTTPRequestHandler):
    server_version = "ADPlatformLFISample/1.0"

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

        if parsed.path == "/v1/render":
            template = parse_qs(parsed.query).get("template", [""])[0]
            if template == "":
                self._send_json(HTTPStatus.BAD_REQUEST, {"status": "missing_template"})
                return

            # Intentionally vulnerable path traversal/LFI:
            # user-controlled template is joined and resolved without base-path guard.
            candidate = (TEMPLATES_DIR / template).resolve()
            try:
                content = candidate.read_text(encoding="utf-8")
            except OSError:
                self._send_json(HTTPStatus.NOT_FOUND, {"status": "template_missing"})
                return

            self._send_json(HTTPStatus.OK, {"status": "rendered", "template": template, "content": content})
            return

        if parsed.path == "/v1/flag":
            supplied = parse_qs(parsed.query).get("value", [""])[0]
            flag = read_flag()
            if supplied and supplied == flag:
                self._send_json(HTTPStatus.OK, {"status": "stored", "flag": supplied})
                return
            self._send_json(HTTPStatus.NOT_FOUND, {"status": "missing"})
            return

        if parsed.path.startswith("/v1/messages/"):
            slot_raw = parsed.path.rsplit("/", 1)[-1]
            if not slot_raw.isdigit():
                self._send_json(HTTPStatus.BAD_REQUEST, {"status": "invalid_slot"})
                return
            slot = int(slot_raw)
            if slot < 0 or slot > 2:
                self._send_json(HTTPStatus.BAD_REQUEST, {"status": "invalid_slot"})
                return
            token = parse_qs(parsed.query).get("token", [""])[0]
            if token == "":
                self._send_json(HTTPStatus.BAD_REQUEST, {"status": "missing_token"})
                return
            payload = read_slot(slot)
            if payload is None:
                self._send_json(HTTPStatus.NOT_FOUND, {"status": "missing_slot"})
                return
            if token != payload.get("token", ""):
                self._send_json(HTTPStatus.FORBIDDEN, {"status": "forbidden"})
                return
            self._send_json(
                HTTPStatus.OK,
                {
                    "status": "ok",
                    "slot": payload.get("slot"),
                    "title": payload.get("title"),
                    "body": payload.get("body"),
                },
            )
            return

        self._send_json(HTTPStatus.NOT_FOUND, {"status": "not_found"})

    def do_POST(self) -> None:
        parsed = urlparse(self.path)
        if parsed.path == "/v1/flag":
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
            return

        if parsed.path != "/v1/messages":
            self._send_json(HTTPStatus.NOT_FOUND, {"status": "not_found"})
            return

        content_length = int(self.headers.get("Content-Length", "0"))
        raw_body = self.rfile.read(content_length)
        try:
            payload = json.loads(raw_body.decode("utf-8") or "{}")
        except json.JSONDecodeError:
            self._send_json(HTTPStatus.BAD_REQUEST, {"status": "invalid_json"})
            return

        slot = payload.get("slot")
        title = payload.get("title", "")
        body = payload.get("body", "")
        if not isinstance(slot, int) or slot < 0 or slot > 2:
            self._send_json(HTTPStatus.BAD_REQUEST, {"status": "invalid_slot"})
            return
        if not isinstance(title, str) or TITLE_RE.match(title) is None:
            self._send_json(HTTPStatus.BAD_REQUEST, {"status": "invalid_title"})
            return
        if not isinstance(body, str) or len(body) == 0 or len(body) > 2048:
            self._send_json(HTTPStatus.BAD_REQUEST, {"status": "invalid_body"})
            return

        stored = write_slot(slot, title, body)
        self._send_json(
            HTTPStatus.OK,
            {"status": "stored", "slot": stored["slot"], "token": stored["token"], "title": stored["title"]},
        )


def main() -> None:
    bootstrap()
    server = ThreadingHTTPServer(("0.0.0.0", SERVICE_PORT), LFISampleHandler)
    server.serve_forever()


if __name__ == "__main__":
    main()

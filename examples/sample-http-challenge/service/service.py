#!/usr/bin/env python3
import json
import os
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import parse_qs, urlparse


STATE_DIR = Path(os.environ.get("AD_STATE_DIR", "/opt/ad/state"))
STATE_PATH = STATE_DIR / "sample-http-state.json"
SERVICE_PORT = int(os.environ.get("PORT", os.environ.get("AD_PLATFORM_SERVICE_PORT", "8080")))
UNLOCK_PROOF = os.environ.get("AD_PLATFORM_UNLOCK_PROOF", "missing-unlock-proof")


def load_state() -> dict[str, str]:
    if not STATE_PATH.exists():
        return {"flag": ""}
    with STATE_PATH.open("r", encoding="utf-8") as handle:
        data = json.load(handle)
    if not isinstance(data, dict):
        return {"flag": ""}
    flag = data.get("flag", "")
    return {"flag": flag if isinstance(flag, str) else ""}


def save_state(state: dict[str, str]) -> None:
    STATE_DIR.mkdir(parents=True, exist_ok=True)
    with STATE_PATH.open("w", encoding="utf-8") as handle:
        json.dump(state, handle)


class SampleHandler(BaseHTTPRequestHandler):
    server_version = "ADPlatformSampleHTTP/1.0"

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
        state = load_state()

        if parsed.path == "/health":
            self._send_json(HTTPStatus.OK, {"status": "ok"})
            return

        if parsed.path == "/unlock":
            self._send_json(HTTPStatus.OK, {"proof": UNLOCK_PROOF})
            return

        if parsed.path == "/flag":
            supplied = parse_qs(parsed.query).get("value", [""])[0]
            if supplied and supplied == state.get("flag", ""):
                self._send_json(HTTPStatus.OK, {"status": "stored", "flag": supplied})
                return
            self._send_json(HTTPStatus.NOT_FOUND, {"status": "missing"})
            return

        if parsed.path == "/leak":
            flag = state.get("flag", "")
            if flag:
                self._send_json(HTTPStatus.OK, {"status": "leaked", "flag": flag})
                return
            self._send_json(HTTPStatus.NOT_FOUND, {"status": "missing"})
            return

        self._send_json(HTTPStatus.NOT_FOUND, {"status": "not_found"})

    def do_POST(self) -> None:
        parsed = urlparse(self.path)
        if parsed.path != "/flag":
            self._send_json(HTTPStatus.NOT_FOUND, {"status": "not_found"})
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

        save_state({"flag": flag})
        self._send_json(HTTPStatus.OK, {"status": "stored", "flag": flag})


def main() -> None:
    server = ThreadingHTTPServer(("0.0.0.0", SERVICE_PORT), SampleHandler)
    server.serve_forever()


if __name__ == "__main__":
    main()

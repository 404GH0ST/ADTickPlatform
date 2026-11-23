#!/usr/bin/env python3
"""Raw-TCP challenge service for the production fleet smoke.

A newline-delimited control protocol lets the checker store and retrieve the
current flag with a token, and a deliberately insecure LEAK command discloses
the stored flag without the token. LEAK is the attack path the smoke uses to
capture another team's current flag; it is intentionally vulnerable and is not
a normal checker path.

Control protocol (one command per line):
  PUT <token> <flag>   store the current flag          -> OK | ERR <reason>
  GET <token>          read the stored flag            -> FLAG <flag> | MISSING | ERR
  PING                 liveness probe                  -> PONG
  LEAK                 insecure disclosure (attack)    -> FLAG <flag> | MISSING
"""
import os
import socketserver
from pathlib import Path

STATE_DIR = Path(os.environ.get("AD_STATE_DIR", "/opt/ad/state"))
STATE_PATH = STATE_DIR / "fleet-tcp-state.flag"
CHECKER_TOKEN = os.environ.get("AD_CHECKER_TOKEN", "fleet-tcp-checker-token")
SERVICE_PORT = int(os.environ.get("PORT", os.environ.get("AD_PLATFORM_SERVICE_PORT", "8080")))
MAX_LINE_BYTES = 4096


def load_flag() -> str:
    try:
        return STATE_PATH.read_text(encoding="utf-8").strip()
    except OSError:
        return ""


def save_flag(flag: str) -> None:
    STATE_DIR.mkdir(parents=True, exist_ok=True)
    STATE_PATH.write_text(flag, encoding="utf-8")


class Handler(socketserver.StreamRequestHandler):
    timeout = 10

    def handle(self) -> None:
        raw = self.rfile.readline(MAX_LINE_BYTES)
        if not raw:
            return
        line = raw.decode("utf-8", errors="replace").strip()
        parts = line.split(" ")
        command = parts[0].upper() if parts else ""

        if command == "PING":
            self._reply("PONG")
            return

        if command == "LEAK":
            flag = load_flag()
            self._reply(f"FLAG {flag}" if flag else "MISSING")
            return

        if command == "GET":
            if len(parts) != 2 or parts[1] != CHECKER_TOKEN:
                self._reply("ERR forbidden")
                return
            flag = load_flag()
            self._reply(f"FLAG {flag}" if flag else "MISSING")
            return

        if command == "PUT":
            if len(parts) < 3 or parts[1] != CHECKER_TOKEN:
                self._reply("ERR forbidden")
                return
            flag = " ".join(parts[2:]).strip()
            if not flag:
                self._reply("ERR missing_flag")
                return
            save_flag(flag)
            self._reply("OK")
            return

        self._reply("ERR unknown_command")

    def _reply(self, message: str) -> None:
        self.wfile.write((message + "\n").encode("utf-8"))


class Server(socketserver.ThreadingTCPServer):
    allow_reuse_address = True
    daemon_threads = True


def main() -> None:
    with Server(("0.0.0.0", SERVICE_PORT), Handler) as server:
        server.serve_forever()


if __name__ == "__main__":
    main()

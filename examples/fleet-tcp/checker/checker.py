#!/usr/bin/env python3
"""Checker for the raw-TCP fleet challenge service.

Drives the newline-delimited control protocol in examples/fleet-tcp/service.
Supports validate/put/get/check and reports service state with the per-run
nonce so the platform can trust liveness markers.
"""
import json
import os
import socket
import sys

CHECKER_TOKEN = os.environ.get("AD_CHECKER_TOKEN", "fleet-tcp-checker-token")
MAX_RESPONSE_BYTES = 1 << 16


def converse(host: str, port: int, command: str) -> str:
    with socket.create_connection((host, port), timeout=5) as sock:
        sock.settimeout(5)
        sock.sendall((command + "\n").encode("utf-8"))
        chunks = []
        total = 0
        while True:
            data = sock.recv(4096)
            if not data:
                break
            chunks.append(data)
            total += len(data)
            if total > MAX_RESPONSE_BYTES:
                raise ValueError("response exceeds checker limit")
            if b"\n" in data:
                break
    return b"".join(chunks).decode("utf-8", errors="replace").strip()


def main() -> int:
    if len(sys.argv) < 2:
        print("usage: checker.py <phase>", file=sys.stderr)
        return 2

    phase = sys.argv[1]
    if phase == "validate":
        print('ADPLATFORM_CHECKER_CAPABILITIES={"service_state":true,"state_nonce":true}')
        print("ok")
        return 0

    host = os.environ["AD_TARGET_HOST"]
    port = int(os.environ["AD_TARGET_PORT"])
    flag = os.environ.get("AD_FLAG", "")

    try:
        if phase == "put":
            response = converse(host, port, f"PUT {CHECKER_TOKEN} {flag}")
            if response != "OK":
                print(f"unexpected put response: {response}", file=sys.stderr)
                return 1
            print(response)
            return 0

        if phase == "get":
            # Never send the expected flag during retrieval; ask the service to
            # disclose what it stored and compare inside the trusted checker.
            response = converse(host, port, f"GET {CHECKER_TOKEN}")
            if response == "MISSING":
                print("stored flag is missing", file=sys.stderr)
                return 42
            if not response.startswith("FLAG "):
                print(f"unexpected get response: {response}", file=sys.stderr)
                return 1
            stored = response[len("FLAG "):]
            if stored != flag:
                print("stored flag does not match the issued flag", file=sys.stderr)
                return 42
            print("FLAG ok")
            return 0

        if phase == "check":
            response = converse(host, port, "PING")
            if response != "PONG":
                print(f"unexpected check response: {response}", file=sys.stderr)
                return 1
            state = {
                "status": "ok",
                "message": "fleet-tcp checker verified the liveness probe",
                "nonce": os.environ["AD_CHECKER_STATE_NONCE"],
            }
            print("ADPLATFORM_SERVICE_STATE=" + json.dumps(state))
            print(response)
            return 0
    except (OSError, ValueError) as exc:
        print(f"checker error: {exc}", file=sys.stderr)
        return 1

    print(f"unsupported phase: {phase}", file=sys.stderr)
    return 2


if __name__ == "__main__":
    raise SystemExit(main())

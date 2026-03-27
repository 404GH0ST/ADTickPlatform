#!/usr/bin/env python3
import json
import os
import sys
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode
from urllib.request import Request, urlopen


def http_request(method: str, url: str, payload: dict | None = None) -> tuple[int, str]:
    data = None
    headers = {}
    if payload is not None:
        data = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"
    request = Request(url, data=data, headers=headers, method=method)
    with urlopen(request, timeout=5) as response:
        return response.status, response.read().decode("utf-8", errors="replace")


def parse_metadata(raw: str) -> dict:
    raw = (raw or "").strip()
    if raw == "":
        return {}
    try:
        parsed = json.loads(raw)
    except json.JSONDecodeError:
        return {}
    if isinstance(parsed, dict):
        return parsed
    return {}


def slot_for_tick(tick_id: int) -> int:
    if tick_id < 0:
        return 0
    return tick_id % 3


def require_json(status: int, body: str) -> dict:
    if status != 200:
        raise ValueError(body)
    payload = json.loads(body)
    if not isinstance(payload, dict):
        raise ValueError("unexpected response payload")
    return payload


def main() -> int:
    if len(sys.argv) < 2:
        print("usage: checker.py <phase>", file=sys.stderr)
        return 2

    phase = sys.argv[1]
    if phase == "validate":
        print("ok")
        return 0

    host = os.environ["AD_TARGET_HOST"]
    port = os.environ["AD_TARGET_PORT"]
    flag = os.environ.get("AD_FLAG", "")
    tick_id = int(os.environ.get("AD_TICK_ID", "0"))
    metadata = parse_metadata(os.environ.get("AD_METADATA", ""))
    base_url = f"http://{host}:{port}"

    try:
        if phase == "put":
            slot = slot_for_tick(tick_id)
            status, body = http_request(
                "POST",
                f"{base_url}/v1/messages",
                {"slot": slot, "title": f"tick-{tick_id}", "body": flag},
            )
            payload = require_json(status, body)
            token = payload.get("token", "")
            if not isinstance(token, str) or token == "":
                print(body, file=sys.stderr)
                return 1
            print(json.dumps({"slot": slot, "token": token}))
            return 0

        if phase == "get":
            if "slot" not in metadata or "token" not in metadata:
                print("missing checker metadata", file=sys.stderr)
                return 1
            slot = int(metadata["slot"])
            token = str(metadata["token"])
            query = urlencode({"token": token})
            status, body = http_request("GET", f"{base_url}/v1/messages/{slot}?{query}")
            payload = require_json(status, body)
            if payload.get("body") != flag:
                print(body, file=sys.stderr)
                return 1
            print(body)
            return 0

        if phase == "check":
            # This phase uses only valid expected user inputs.
            status, body = http_request("GET", f"{base_url}/v1/health")
            health = require_json(status, body)
            if health.get("status") != "ok":
                print(body, file=sys.stderr)
                return 1

            status, body = http_request("GET", f"{base_url}/v1/render?{urlencode({'template': 'welcome.txt'})}")
            render = require_json(status, body)
            if "Welcome to sample LFI challenge." not in str(render.get("content", "")):
                print(body, file=sys.stderr)
                return 1

            slot = (slot_for_tick(tick_id) + 1) % 3
            status, body = http_request(
                "POST",
                f"{base_url}/v1/messages",
                {"slot": slot, "title": "health-check", "body": "normal user content"},
            )
            payload = require_json(status, body)
            token = payload.get("token", "")
            if not isinstance(token, str) or token == "":
                print(body, file=sys.stderr)
                return 1

            status, body = http_request("GET", f"{base_url}/v1/messages/{slot}?{urlencode({'token': token})}")
            payload = require_json(status, body)
            if payload.get("body") != "normal user content":
                print(body, file=sys.stderr)
                return 1
            print(json.dumps({"status": "ok", "slot": slot}))
            return 0
    except (HTTPError, URLError, ValueError, json.JSONDecodeError) as exc:
        print(f"checker error: {exc}", file=sys.stderr)
        return 1

    print(f"unsupported phase: {phase}", file=sys.stderr)
    return 2


if __name__ == "__main__":
    raise SystemExit(main())

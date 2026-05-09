#!/usr/bin/env python3
import json
import os
import sys
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode
from urllib.request import Request, urlopen


CHECKER_TOKEN = os.environ.get("AD_CHECKER_TOKEN", "sample-lfi-checker-token")


def http_request(
    method: str,
    url: str,
    payload: dict | None = None,
    headers: dict[str, str] | None = None,
) -> tuple[int, str]:
    data = None
    request_headers = {}
    if payload is not None:
        data = json.dumps(payload).encode("utf-8")
        request_headers["Content-Type"] = "application/json"
    if headers:
        request_headers.update(headers)
    request = Request(url, data=data, headers=request_headers, method=method)
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
    metadata = parse_metadata(os.environ.get("AD_METADATA", ""))
    base_url = f"http://{host}:{port}"

    try:
        if phase == "put":
            status, body = http_request(
                "POST",
                f"{base_url}/v1/flag",
                {"flag": flag},
                {"X-Checker-Token": CHECKER_TOKEN},
            )
            payload = require_json(status, body)
            if payload.get("path") != "flag.txt":
                print(body, file=sys.stderr)
                return 1
            print(json.dumps({"path": "flag.txt"}))
            return 0

        if phase == "get":
            if metadata.get("path") != "flag.txt":
                print("missing checker metadata", file=sys.stderr)
                return 1
            query = urlencode({"value": flag})
            status, body = http_request("GET", f"{base_url}/v1/flag?{query}")
            payload = require_json(status, body)
            if payload.get("flag") != flag:
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

            slot = 0
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
            print('ADPLATFORM_SERVICE_STATE={"status":"ok","message":"sample-lfi checker verified storage, retrieval, and functionality"}')
            print(json.dumps({"status": "ok", "slot": slot}))
            return 0
    except (HTTPError, URLError, ValueError, json.JSONDecodeError) as exc:
        print(f"checker error: {exc}", file=sys.stderr)
        return 1

    print(f"unsupported phase: {phase}", file=sys.stderr)
    return 2


if __name__ == "__main__":
    raise SystemExit(main())

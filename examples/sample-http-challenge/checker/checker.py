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
    base_url = f"http://{host}:{port}"

    try:
        if phase == "put":
            status, body = http_request("POST", f"{base_url}/flag", {"flag": flag})
            if status != 200:
                print(body, file=sys.stderr)
                return 1
            print(body)
            return 0

        if phase == "get":
            query = urlencode({"value": flag})
            status, body = http_request("GET", f"{base_url}/flag?{query}")
            if status != 200:
                print(body, file=sys.stderr)
                return 1
            print(body)
            return 0

        if phase == "check":
            status, body = http_request("GET", f"{base_url}/health")
            if status != 200:
                print(body, file=sys.stderr)
                return 1
            print('ADPLATFORM_SERVICE_STATE={"status":"ok","message":"sample-http checker verified the public health route"}')
            print(body)
            return 0
    except HTTPError as exc:
        print(exc.read().decode("utf-8", errors="replace"), file=sys.stderr)
        return 1
    except URLError as exc:
        print(f"request failed: {exc}", file=sys.stderr)
        return 1

    print(f"unsupported phase: {phase}", file=sys.stderr)
    return 2


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
import json
import os
import sys
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen


CHECKER_TOKEN = os.environ.get("AD_CHECKER_TOKEN", "sample-http-checker-token")
MAX_HTTP_RESPONSE_BYTES = 1 << 20


def read_response_body(response) -> str:
    data = response.read(MAX_HTTP_RESPONSE_BYTES + 1)
    if len(data) > MAX_HTTP_RESPONSE_BYTES:
        raise ValueError("response body exceeds checker limit")
    return data.decode("utf-8", errors="replace")


def http_request(method: str, url: str, payload: dict | None = None, headers: dict | None = None) -> tuple[int, str]:
    data = None
    headers = dict(headers or {})
    if payload is not None:
        data = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"
    request = Request(url, data=data, headers=headers, method=method)
    with urlopen(request, timeout=5) as response:
        return response.status, read_response_body(response)


def main() -> int:
    if len(sys.argv) < 2:
        print("usage: checker.py <phase>", file=sys.stderr)
        return 2

    phase = sys.argv[1]
    if phase == "validate":
        print('ADPLATFORM_CHECKER_CAPABILITIES={"service_state":true}')
        print("ok")
        return 0

    host = os.environ["AD_TARGET_HOST"]
    port = os.environ["AD_TARGET_PORT"]
    flag = os.environ.get("AD_FLAG", "")
    base_url = f"http://{host}:{port}"

    try:
        if phase == "put":
            status, body = http_request("POST", f"{base_url}/flag", {"flag": flag}, {"X-Checker-Token": CHECKER_TOKEN})
            if status != 200:
                print(body, file=sys.stderr)
                return 1
            print(body)
            return 0

        if phase == "get":
            # Never send the expected flag to the service during retrieval.
            try:
                status, body = http_request("GET", f"{base_url}/flag", headers={"X-Checker-Token": CHECKER_TOKEN})
            except HTTPError as exc:
                if exc.code == 404:
                    missing = json.loads(read_response_body(exc))
                    if isinstance(missing, dict) and missing.get("status") == "missing":
                        print("stored flag is missing", file=sys.stderr)
                        return 42
                raise
            payload = json.loads(body)
            if status != 200:
                return 1
            if not isinstance(payload, dict) or payload.get("flag") != flag:
                print("stored flag does not match the issued flag", file=sys.stderr)
                return 42
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
        try:
            message = read_response_body(exc)
        except ValueError as read_error:
            message = str(read_error)
        print(message, file=sys.stderr)
        return 1
    except URLError as exc:
        print(f"request failed: {exc}", file=sys.stderr)
        return 1
    except ValueError as exc:
        print(f"checker error: {exc}", file=sys.stderr)
        return 1

    print(f"unsupported phase: {phase}", file=sys.stderr)
    return 2


if __name__ == "__main__":
    raise SystemExit(main())

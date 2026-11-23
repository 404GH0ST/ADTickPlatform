#!/usr/bin/env python3
"""Flask web challenge service for the production fleet smoke.

Implements the same HTTP contract as examples/sample-http-challenge so the
bundled sample-http checker can drive put/get/check against it. The /leak
route is the deliberately insecure attack path used by the smoke to capture
another team's current flag.
"""
import json
import os
from pathlib import Path

from flask import Flask, jsonify, request

STATE_DIR = Path(os.environ.get("AD_STATE_DIR", "/opt/ad/state"))
STATE_PATH = STATE_DIR / "fleet-web-flask-state.json"
CHECKER_TOKEN = os.environ.get("AD_CHECKER_TOKEN", "sample-http-checker-token")
SERVICE_PORT = int(os.environ.get("PORT", os.environ.get("AD_PLATFORM_SERVICE_PORT", "8080")))

app = Flask(__name__)


def load_flag() -> str:
    if not STATE_PATH.exists():
        return ""
    try:
        data = json.loads(STATE_PATH.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return ""
    flag = data.get("flag", "") if isinstance(data, dict) else ""
    return flag if isinstance(flag, str) else ""


def save_flag(flag: str) -> None:
    STATE_DIR.mkdir(parents=True, exist_ok=True)
    STATE_PATH.write_text(json.dumps({"flag": flag}), encoding="utf-8")


@app.get("/health")
def health():
    return jsonify(status="ok")


@app.get("/flag")
def get_flag():
    if request.headers.get("X-Checker-Token", "") != CHECKER_TOKEN:
        return jsonify(status="forbidden"), 403
    flag = load_flag()
    if flag:
        return jsonify(status="stored", flag=flag)
    return jsonify(status="missing"), 404


@app.post("/flag")
def put_flag():
    if request.headers.get("X-Checker-Token", "") != CHECKER_TOKEN:
        return jsonify(status="forbidden"), 403
    payload = request.get_json(silent=True)
    if not isinstance(payload, dict):
        return jsonify(status="invalid_json"), 400
    flag = payload.get("flag", "")
    if not isinstance(flag, str) or flag == "":
        return jsonify(status="missing_flag"), 400
    save_flag(flag)
    return jsonify(status="stored", flag=flag)


@app.get("/leak")
def leak():
    flag = load_flag()
    if flag:
        return jsonify(status="leaked", flag=flag)
    return jsonify(status="missing"), 404


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=SERVICE_PORT, threaded=True)

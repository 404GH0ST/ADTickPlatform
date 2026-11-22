#!/usr/bin/env node
// Node.js web challenge service for the production fleet smoke.
// Implements the same HTTP contract as examples/sample-http-challenge so the
// bundled sample-http checker can drive put/get/check against it. The /leak
// route is the deliberately insecure attack path used by the smoke.
"use strict";

const http = require("http");
const fs = require("fs");
const path = require("path");

const STATE_DIR = process.env.AD_STATE_DIR || "/opt/ad/state";
const STATE_PATH = path.join(STATE_DIR, "fleet-web-node-state.json");
const CHECKER_TOKEN = process.env.AD_CHECKER_TOKEN || "sample-http-checker-token";
const SERVICE_PORT = parseInt(
  process.env.PORT || process.env.AD_PLATFORM_SERVICE_PORT || "8080",
  10
);
const MAX_BODY_BYTES = 1 << 20;

function loadFlag() {
  try {
    const data = JSON.parse(fs.readFileSync(STATE_PATH, "utf-8"));
    if (data && typeof data.flag === "string") {
      return data.flag;
    }
  } catch (err) {
    /* missing or malformed state means no flag yet */
  }
  return "";
}

function saveFlag(flag) {
  fs.mkdirSync(STATE_DIR, { recursive: true });
  fs.writeFileSync(STATE_PATH, JSON.stringify({ flag }), "utf-8");
}

function sendJson(res, status, payload) {
  const body = Buffer.from(JSON.stringify(payload), "utf-8");
  res.writeHead(status, {
    "Content-Type": "application/json",
    "Content-Length": body.length,
  });
  res.end(body);
}

const server = http.createServer((req, res) => {
  const url = new URL(req.url, "http://localhost");
  const route = url.pathname;

  if (req.method === "GET" && route === "/health") {
    sendJson(res, 200, { status: "ok" });
    return;
  }

  if (req.method === "GET" && route === "/flag") {
    if (req.headers["x-checker-token"] !== CHECKER_TOKEN) {
      sendJson(res, 403, { status: "forbidden" });
      return;
    }
    const flag = loadFlag();
    if (flag) {
      sendJson(res, 200, { status: "stored", flag });
    } else {
      sendJson(res, 404, { status: "missing" });
    }
    return;
  }

  if (req.method === "GET" && route === "/leak") {
    const flag = loadFlag();
    if (flag) {
      sendJson(res, 200, { status: "leaked", flag });
    } else {
      sendJson(res, 404, { status: "missing" });
    }
    return;
  }

  if (req.method === "POST" && route === "/flag") {
    if (req.headers["x-checker-token"] !== CHECKER_TOKEN) {
      sendJson(res, 403, { status: "forbidden" });
      return;
    }
    const chunks = [];
    let size = 0;
    let aborted = false;
    req.on("data", (chunk) => {
      size += chunk.length;
      if (size > MAX_BODY_BYTES) {
        aborted = true;
        sendJson(res, 413, { status: "too_large" });
        req.destroy();
        return;
      }
      chunks.push(chunk);
    });
    req.on("end", () => {
      if (aborted) {
        return;
      }
      let payload;
      try {
        payload = JSON.parse(Buffer.concat(chunks).toString("utf-8") || "{}");
      } catch (err) {
        sendJson(res, 400, { status: "invalid_json" });
        return;
      }
      const flag = payload && payload.flag;
      if (typeof flag !== "string" || flag === "") {
        sendJson(res, 400, { status: "missing_flag" });
        return;
      }
      saveFlag(flag);
      sendJson(res, 200, { status: "stored", flag });
    });
    return;
  }

  sendJson(res, 404, { status: "not_found" });
});

server.listen(SERVICE_PORT, "0.0.0.0");

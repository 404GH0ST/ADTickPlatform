#!/usr/bin/env python3
"""Generate a participant-facing API reference (docx) from the OpenAPI spec.

Outputs docs/participant-api-reference.docx with one section per
participant endpoint. Admin (/api/v2/admin/) and internal
(/internal/v1/) endpoints are excluded — participants never see
those.

Re-run after editing docs/platform-api-v2.openapi.yaml:
  python3 scripts/generate-participant-api-doc.py
"""

from pathlib import Path

import yaml
from docx import Document
from docx.enum.table import WD_ALIGN_VERTICAL
from docx.shared import Pt, RGBColor

SPEC_PATH = Path("docs/platform-api-v2.openapi.yaml")
OUT_PATH = Path("docs/participant-api-reference.docx")

EXCLUDE_PATH_PREFIXES = ("/api/v2/admin/", "/internal/v1/", "/internal/")


def is_participant_endpoint(path: str) -> bool:
    if not path.startswith("/api/v2/"):
        return False
    return not any(path.startswith(prefix) for prefix in EXCLUDE_PATH_PREFIXES)


def auth_required_for_path(path: str) -> str:
    if path == "/api/v2/authenticate":
        return "No"
    return "Yes (Authorization: Bearer <token>)"


def short_type(t: str) -> str:
    mapping = {"string": "str", "integer": "int", "boolean": "bool", "number": "num", "array": "[]"}
    return mapping.get(t, t)


def render_param_table(doc, params: list[dict]) -> None:
    if not params:
        doc.add_paragraph("(none)").italic = True
        return
    table = doc.add_table(rows=1 + len(params), cols=4)
    table.style = "Light Grid Accent 1"
    for cell, header in zip(table.rows[0].cells, ("Name", "In", "Type", "Description")):
        cell.text = header
        for paragraph in cell.paragraphs:
            for run in paragraph.runs:
                run.bold = True
    for row, param in enumerate(params, start=1):
        cells = table.rows[row].cells
        cells[0].text = param.get("name", "")
        cells[1].text = param.get("in", "")
        cells[2].text = short_type(str(param.get("schema", {}).get("type", "—")))
        cells[3].text = param.get("description", "") or "—"
    _autosize_table(table)


def render_body_table(doc, body: dict) -> None:
    schema = body.get("schema", {})
    if not schema:
        doc.add_paragraph("(none)").italic = True
        return
    properties = schema.get("properties", {})
    required = set(schema.get("required", []))
    if not properties:
        doc.add_paragraph("(empty body)").italic = True
        return
    table = doc.add_table(rows=1 + len(properties), cols=4)
    table.style = "Light Grid Accent 1"
    for cell, header in zip(table.rows[0].cells, ("Field", "Type", "Required", "Description")):
        cell.text = header
        for paragraph in cell.paragraphs:
            for run in paragraph.runs:
                run.bold = True
    for row, (name, prop) in enumerate(properties.items(), start=1):
        cells = table.rows[row].cells
        cells[0].text = name
        prop_type = prop.get("type", "")
        if prop_type == "array":
            prop_type = f"[]{short_type(prop.get('items', {}).get('type', '?'))}"
        cells[1].text = short_type(prop_type)
        cells[2].text = "yes" if name in required else "no"
        cells[3].text = prop.get("description", "") or "—"
    _autosize_table(table)


def render_response_table(doc, responses: dict) -> None:
    table = doc.add_table(rows=1 + len(responses), cols=2)
    table.style = "Light Grid Accent 1"
    for cell, header in zip(table.rows[0].cells, ("Status", "Meaning")):
        cell.text = header
        for paragraph in cell.paragraphs:
            for run in paragraph.runs:
                run.bold = True
    for row, (status, body) in enumerate(responses.items(), start=1):
        cells = table.rows[row].cells
        cells[0].text = str(status)
        cells[1].text = body.get("description", "") or "—"
    _autosize_table(table)


def render_endpoint(doc, path: str, method: str, op: dict) -> None:
    heading = doc.add_paragraph()
    heading.style = "Heading 2"
    run = heading.add_run(f"{method.upper()} {path}")
    run.bold = True
    run.font.size = Pt(14)

    if op.get("summary"):
        doc.add_paragraph(op["summary"]).italic = True
    if op.get("description") and op["description"] != op.get("summary"):
        doc.add_paragraph(op["description"])

    info_table = doc.add_table(rows=4, cols=2)
    info_table.style = "Light Grid Accent 1"
    info_rows = [
        ("Authentication", auth_required_for_path(path)),
        ("Method", method.upper()),
        ("Path", path),
        ("Operation ID", op.get("operationId", "—")),
    ]
    for row, (k, v) in enumerate(info_rows):
        cells = info_table.rows[row].cells
        cells[0].text = k
        for paragraph in cells[0].paragraphs:
            for run in paragraph.runs:
                run.bold = True
        cells[1].text = v
    _autosize_table(info_table)

    doc.add_paragraph().add_run("Path / Query parameters").bold = True
    render_param_table(doc, op.get("parameters", []))

    if "requestBody" in op:
        doc.add_paragraph().add_run("Request body").bold = True
        render_body_table(doc, op["requestBody"])

    doc.add_paragraph().add_run("Responses").bold = True
    render_response_table(doc, op.get("responses", {}))

    doc.add_paragraph("—" * 40)


def _autosize_table(table) -> None:
    for column in table.columns:
        for cell in column.cells:
            for paragraph in cell.paragraphs:
                paragraph.paragraph_format.space_after = Pt(2)


def main() -> int:
    with SPEC_PATH.open() as f:
        spec = yaml.safe_load(f)

    info = spec.get("info", {})
    servers = spec.get("servers", [])
    components = spec.get("components", {}).get("schemas", {})

    doc = Document()
    title = doc.add_paragraph()
    title_run = title.add_run("ADTickPlatform — Participant API Reference")
    title_run.bold = True
    title_run.font.size = Pt(20)

    doc.add_paragraph(
        f"Version: {info.get('version', '—')}  •  "
        f"Generated from docs/platform-api-v2.openapi.yaml"
    )
    if info.get("description"):
        doc.add_paragraph(info["description"])

    doc.add_heading("1. Overview", level=1)
    doc.add_paragraph(
        "This document describes every public endpoint a participant can call during an "
        "Attack-Defense tick. Admin endpoints under /api/v2/admin/ and internal service-to-service "
        "endpoints under /internal/v1/ are excluded. Authentication is a 24-hour bearer token "
        "issued by POST /api/v2/authenticate; send it in every subsequent request as "
        "`Authorization: Bearer <token>`."
    )
    doc.add_paragraph(
        "All endpoints return problem+json on non-2xx (RFC 7807). Rate limits are enforced per "
        "team (most endpoints) and per user (submit only). Exceeding a limit returns 429 with a "
        "Retry-After header."
    )

    if servers:
        doc.add_paragraph("Base URLs (per environment):").bold = True
        for server in servers:
            description = server.get("description", "")
            doc.add_paragraph(f"  • {server.get('url', '—')}  —  {description}", style="List Bullet")

    doc.add_paragraph("Common request headers:").bold = True
    headers_table = doc.add_table(rows=4, cols=2)
    headers_table.style = "Light Grid Accent 1"
    for cell, header in zip(headers_table.rows[0].cells, ("Header", "Purpose")):
        cell.text = header
        for paragraph in cell.paragraphs:
            for run in paragraph.runs:
                run.bold = True
    for row, (k, v) in enumerate([
        ("Authorization", "Bearer <token> (required for every endpoint except POST /api/v2/authenticate)"),
        ("Content-Type", "application/json (for POST/PUT bodies)"),
        ("Accept", "application/json (default); problem+json is returned for errors"),
    ], start=1):
        headers_table.rows[row].cells[0].text = k
        headers_table.rows[row].cells[1].text = v
    _autosize_table(headers_table)

    doc.add_paragraph("Common error responses:").bold = True
    error_table = doc.add_table(rows=6, cols=2)
    error_table.style = "Light Grid Accent 1"
    for cell, header in zip(error_table.rows[0].cells, ("Status", "When")):
        cell.text = header
        for paragraph in cell.paragraphs:
            for run in paragraph.runs:
                run.bold = True
    for row, (k, v) in enumerate([
        ("400", "Request body is malformed or missing required fields"),
        ("401", "(reserved) admin-only paths return 403, see OpenAPI"),
        ("403", "Missing or invalid bearer token (or organizer token used on a participant path)"),
        ("404", "Resource not found (challenge ID, etc.)"),
        ("429", "Rate limit exceeded; honor the Retry-After header"),
    ], start=1):
        error_table.rows[row].cells[0].text = k
        error_table.rows[row].cells[1].text = v
    _autosize_table(error_table)

    doc.add_heading("2. Endpoints", level=1)
    paths = spec.get("paths", {})

    method_order = {"get": 0, "post": 1, "put": 2, "patch": 3, "delete": 4}

    grouped: dict[str, list[tuple[str, str, dict]]] = {}
    for path in sorted(paths.keys()):
        if not is_participant_endpoint(path):
            continue
        for method in sorted(paths[path].keys(), key=lambda m: method_order.get(m, 99)):
            op = paths[path][method]
            if not isinstance(op, dict) or "responses" not in op:
                continue
            category = _category_for_path(path)
            grouped.setdefault(category, []).append((path, method, op))

    if "session" not in {p for p, _, _ in grouped.get("auth", [])}:
        doc.add_heading("3. Note on missing /api/v2/session", level=1)
        doc.add_paragraph(
            "The current OpenAPI spec does not document GET /api/v2/session, even though the "
            "server implements it (used by the web frontend to validate a stored token on page "
            "load). Treat it as reserved — the spec is the source of truth and will catch up in "
            "the next regeneration."
        )

    category_titles = {
        "auth": "Authentication & session",
        "me": "Self-service (VPN, profile)",
        "challenges": "Challenge metadata & source",
        "services": "Service state & operations",
        "team": "Team-scoped views",
        "scoreboard": "Public scoreboard & attack feed",
        "submit": "Flag submission",
    }

    for category, endpoints in grouped.items():
        doc.add_heading(f"  {category_titles.get(category, category.capitalize())}", level=2)
        for path, method, op in endpoints:
            render_endpoint(doc, path, method, op)

    doc.add_heading(f"{len(grouped) + 2}. Schema reference", level=1)
    doc.add_paragraph(
        "The following component schemas are referenced by the endpoints above. They are "
        "documented here for completeness; refer to the live spec for the authoritative definition."
    )
    interesting = [
        "submitRequest", "submitRequestFlags", "submitResult", "submitResultResultEntry",
        "challengeSummary", "challenge", "serviceState", "scoreRow", "scoreServiceRow",
        "attackEvent", "gameStatus", "gameMatchStatus", "meWireguardConfig",
        "errorProblem",
    ]
    for schema_name in interesting:
        if schema_name not in components:
            continue
        schema = components[schema_name]
        doc.add_heading(schema_name, level=3)
        table = doc.add_table(rows=1, cols=2)
        table.style = "Light Grid Accent 1"
        for cell, header in zip(table.rows[0].cells, ("Field", "Type")):
            cell.text = header
            for paragraph in cell.paragraphs:
                for run in paragraph.runs:
                    run.bold = True
        required = set(schema.get("required", []))
        for row, (name, prop) in enumerate(schema.get("properties", {}).items(), start=1):
            table.add_row()
            cells = table.rows[row].cells
            cells[0].text = f"{name}{'*' if name in required else ''}"
            prop_type = prop.get("type", "")
            if prop_type == "array":
                prop_type = f"[]{short_type(prop.get('items', {}).get('type', '?'))}"
            elif "$ref" in prop:
                prop_type = prop["$ref"].rsplit("/", 1)[-1]
            cells[1].text = short_type(prop_type)
        _autosize_table(table)
        doc.add_paragraph("*" + " marks required fields.").italic = True

    doc.save(OUT_PATH)
    print(f"wrote {OUT_PATH}  ({OUT_PATH.stat().st_size:,} bytes)")
    return 0


def _category_for_path(path: str) -> str:
    if path == "/api/v2/authenticate":
        return "auth"
    if path.startswith("/api/v2/me/"):
        return "me"
    if path.startswith("/api/v2/challenges"):
        return "challenges"
    if path.startswith("/api/v2/services") or "/api/v2/team/services" in path:
        return "services" if path.startswith("/api/v2/services") else "team"
    if path in ("/api/v2/scoreboard", "/api/v2/attacks", "/api/v2/game/status"):
        return "scoreboard"
    if "/api/v2/team/services" in path:
        return "team"
    if path == "/api/v2/submit":
        return "submit"
    return "other"


if __name__ == "__main__":
    raise SystemExit(main())

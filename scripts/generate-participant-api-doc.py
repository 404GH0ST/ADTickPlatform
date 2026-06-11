#!/usr/bin/env python3
"""Generate a participant-facing API reference (docx) from the OpenAPI spec.

Output: docs/participant-api-reference.docx

Design choices:
  - Calibri family throughout (renders correctly in Word, LibreOffice,
    Google Docs, Pages) — no exotic fonts that might be substituted.
  - Slate-gray palette + warm amber accent. Explicitly NO blue anywhere
    (avoids the default-Office look). Status codes get semantic
    colors: emerald-700 for 2xx, amber-700 for 4xx, red-700 for 5xx.
  - Cover page, table of contents, page numbers in the footer.
  - Per-endpoint block uses a definition-list table (Field/Detail) for
    auth + params + body + responses, with a subtle top border to
    visually separate each endpoint.
  - Alternating row backgrounds on data tables for scannability.
  - Monospace example payloads in a shaded paragraph block.

Re-run after editing docs/platform-api-v2.openapi.yaml:
  python3 scripts/generate-participant-api-doc.py
"""

from pathlib import Path

import yaml
from docx import Document
from docx.enum.table import WD_ALIGN_VERTICAL
from docx.enum.text import WD_ALIGN_PARAGRAPH
from docx.oxml import OxmlElement
from docx.oxml.ns import qn
from docx.shared import Pt, RGBColor

SPEC_PATH = Path("docs/platform-api-v2.openapi.yaml")
OUT_PATH = Path("docs/participant-api-reference.docx")

EXCLUDE_PATH_PREFIXES = ("/api/v2/admin/", "/internal/v1/", "/internal/")

# Slate grays + warm amber. No blue on purpose.
INK = RGBColor(0x0F, 0x17, 0x2A)        # slate-950, primary text
BODY = RGBColor(0x1F, 0x29, 0x37)       # slate-800, body text
MUTED = RGBColor(0x64, 0x74, 0x8B)       # slate-500, secondary text
SOFT = RGBColor(0x94, 0xA3, 0xB8)        # slate-400, subtle
RULE = RGBColor(0xCB, 0xD5, 0xE1)        # slate-300, rules
ROW_ALT = "F1F5F9"                       # slate-100, alt row
CODE_BG = "F8FAFC"                       # slate-50, code block

OK_2XX = RGBColor(0x04, 0x78, 0x57)        # emerald-700 (text)
WARN_4XX = RGBColor(0xB4, 0x53, 0x09)     # amber-700 (text)
ERR_5XX = RGBColor(0xB9, 0x1C, 0x1C)      # red-700 (text)

# Hex strings for cell shading (python-docx set_cell_shading wants strings)
OK_2XX_HEX = "047857"
WARN_4XX_HEX = "B45309"
ERR_5XX_HEX = "B91C1C"

FONT = "Calibri"

STATUS_COLOR = {
    "200": OK_2XX_HEX, "201": OK_2XX_HEX, "202": OK_2XX_HEX, "204": OK_2XX_HEX,
    "400": WARN_4XX_HEX, "401": WARN_4XX_HEX, "403": WARN_4XX_HEX, "404": WARN_4XX_HEX, "409": WARN_4XX_HEX, "429": WARN_4XX_HEX,
    "500": ERR_5XX_HEX, "502": ERR_5XX_HEX, "503": ERR_5XX_HEX,
}


def is_participant(path: str) -> bool:
    return path.startswith("/api/v2/") and not any(path.startswith(p) for p in EXCLUDE_PATH_PREFIXES)


def short_type(t: str) -> str:
    return {"string": "str", "integer": "int", "boolean": "bool", "number": "num", "array": "[]"}.get(t, t)


def set_cell_shading(cell, hex_color: str) -> None:
    tc_pr = cell._tc.get_or_add_tcPr()
    shd = OxmlElement("w:shd")
    shd.set(qn("w:val"), "clear")
    shd.set(qn("w:color"), "auto")
    shd.set(qn("w:fill"), hex_color)
    tc_pr.append(shd)


def set_cell_borders(cell, hex_color: str = "CBD5E1", size: int = 4) -> None:
    tc_pr = cell._tc.get_or_add_tcPr()
    tc_borders = OxmlElement("w:tcBorders")
    for edge in ("top", "left", "bottom", "right"):
        b = OxmlElement(f"w:{edge}")
        b.set(qn("w:val"), "single")
        b.set(qn("w:sz"), str(size))
        b.set(qn("w:color"), hex_color)
        tc_borders.append(b)
    tc_pr.append(tc_borders)


def set_paragraph_shading(paragraph, hex_color: str) -> None:
    p_pr = paragraph._p.get_or_add_pPr()
    shd = OxmlElement("w:shd")
    shd.set(qn("w:val"), "clear")
    shd.set(qn("w:color"), "auto")
    shd.set(qn("w:fill"), hex_color)
    p_pr.append(shd)


def add_horizontal_line(paragraph, hex_color: str = "CBD5E1") -> None:
    p_pr = paragraph._p.get_or_add_pPr()
    p_bdr = OxmlElement("w:pBdr")
    bottom = OxmlElement("w:bottom")
    bottom.set(qn("w:val"), "single")
    bottom.set(qn("w:sz"), "6")
    bottom.set(qn("w:space"), "1")
    bottom.set(qn("w:color"), hex_color)
    p_bdr.append(bottom)
    p_pr.append(p_bdr)


def set_run_font(run, size_pt: float, *, bold: bool = False, italic: bool = False,
                 color: RGBColor | None = None, font_name: str = FONT) -> None:
    run.font.name = font_name
    run.font.size = Pt(size_pt)
    run.font.bold = bold
    run.font.italic = italic
    if color is not None:
        run.font.color.rgb = color


def style_table_borders(table, hex_color: str = "CBD5E1") -> None:
    for row in table.rows:
        for cell in row.cells:
            set_cell_borders(cell, hex_color)


def apply_alt_rows(table) -> None:
    for idx, row in enumerate(table.rows):
        if idx % 2 == 1:
            for cell in row.cells:
                set_cell_shading(cell, ROW_ALT)


def write_text(cell, text: str, *, size_pt: float = 10, bold: bool = False,
               italic: bool = False, color: RGBColor = BODY) -> None:
    cell.text = ""
    para = cell.paragraphs[0]
    run = para.add_run(text)
    set_run_font(run, size_pt, bold=bold, italic=italic, color=color)


def auth_required_for_path(path: str) -> str:
    return "No" if path == "/api/v2/authenticate" else "Bearer <token> (required)"


def category_for_path(path: str) -> str:
    if path == "/api/v2/authenticate" or path == "/api/v2/session":
        return "auth"
    if path.startswith("/api/v2/me/"):
        return "me"
    if path.startswith("/api/v2/challenges"):
        return "challenges"
    if path == "/api/v2/team/services":
        return "team"
    if path.startswith("/api/v2/services"):
        return "services"
    if path in ("/api/v2/scoreboard", "/api/v2/attacks", "/api/v2/game/status"):
        return "scoreboard"
    if path == "/api/v2/submit":
        return "submit"
    return "other"


CATEGORY_TITLES = {
    "auth": "Authentication",
    "me": "Self-service",
    "challenges": "Challenges",
    "services": "Services",
    "team": "Team views",
    "scoreboard": "Scoreboard & attack feed",
    "submit": "Flag submission",
}


def add_cover_page(doc, info: dict) -> None:
    for _ in range(4):
        doc.add_paragraph()
    title = doc.add_paragraph()
    title.alignment = WD_ALIGN_PARAGRAPH.LEFT
    run = title.add_run("ADTickPlatform")
    set_run_font(run, 32, bold=False, color=INK)

    sub = doc.add_paragraph()
    sub.alignment = WD_ALIGN_PARAGRAPH.LEFT
    run = sub.add_run("Participant API Reference")
    set_run_font(run, 20, bold=False, color=MUTED)

    rule = doc.add_paragraph()
    add_horizontal_line(rule, "94A3B8")

    meta = doc.add_paragraph()
    meta.paragraph_format.space_before = Pt(18)
    run = meta.add_run(f"Version {info.get('version', '—')}")
    set_run_font(run, 11, color=BODY)
    meta.add_run("\n")
    run = meta.add_run("Compiled from docs/platform-api-v2.openapi.yaml")
    set_run_font(run, 11, color=MUTED)
    meta.add_run("\n")
    run = meta.add_run("Regenerate with: python3 scripts/generate-participant-api-doc.py")
    set_run_font(run, 10, italic=True, color=SOFT)

    if info.get("description"):
        desc = doc.add_paragraph()
        desc.paragraph_format.space_before = Pt(18)
        run = desc.add_run(info["description"])
        set_run_font(run, 10, color=MUTED)


def add_toc(doc, paths: dict) -> None:
    doc.add_page_break()
    heading = doc.add_paragraph()
    run = heading.add_run("Table of contents")
    set_run_font(run, 20, bold=False, color=INK)

    sections = [
        ("1. Overview", 3),
        ("2. Common conventions", 3),
        ("3. Endpoints", 2),
    ]
    grouped: dict[str, list[tuple[str, str, str]]] = {}
    for path in sorted(paths.keys()):
        if not is_participant(path):
            continue
        for method in sorted(paths[path].keys()):
            op = paths[path][method]
            if not isinstance(op, dict) or "responses" not in op:
                continue
            category = category_for_path(path)
            grouped.setdefault(category, []).append((method.upper(), path, op.get("summary", "")))

    for label, level in sections:
        para = doc.add_paragraph()
        para.paragraph_format.space_before = Pt(12 if level == 2 else 6)
        run = para.add_run(label)
        set_run_font(run, 12 if level == 2 else 10, bold=level == 2, color=INK)

    for category in ["auth", "me", "challenges", "services", "team", "scoreboard", "submit"]:
        if category not in grouped:
            continue
        para = doc.add_paragraph()
        para.paragraph_format.space_before = Pt(4)
        para.paragraph_format.left_indent = Pt(12)
        run = para.add_run(CATEGORY_TITLES[category])
        set_run_font(run, 10, bold=True, color=MUTED)

        for method, path, summary in grouped[category]:
            line = doc.add_paragraph()
            line.paragraph_format.left_indent = Pt(24)
            line.paragraph_format.space_after = Pt(0)
            run = line.add_run(f"  {method} {path}")
            set_run_font(run, 9, color=BODY)
            if summary:
                run = line.add_run(f"  — {summary}")
                set_run_font(run, 9, italic=True, color=SOFT)

    para = doc.add_paragraph()
    para.paragraph_format.space_before = Pt(12)
    run = para.add_run("4. Schema reference")
    set_run_font(run, 12, bold=True, color=INK)


def add_overview(doc, spec: dict) -> None:
    doc.add_page_break()
    heading = doc.add_paragraph()
    run = heading.add_run("1. Overview")
    set_run_font(run, 22, bold=False, color=INK)

    for text in [
        "This document describes every public endpoint a participant can call during an "
        "Attack-Defense tick. Admin endpoints under /api/v2/admin/ and internal service-to-service "
        "endpoints under /internal/v1/ are excluded.",
        "Authentication is a 24-hour bearer token issued by POST /api/v2/authenticate; send it in "
        "every subsequent request as Authorization: Bearer <token>. The token is re-validated "
        "against the store on every request, so a deleted player returns 403 even if the JWT is "
        "still valid.",
        "All endpoints return problem+json on non-2xx (RFC 7807). Rate limits are enforced per "
        "team for most endpoints, with an extra per-user layer on /api/v2/submit. Exceeding a "
        "limit returns 429 with a Retry-After header.",
    ]:
        p = doc.add_paragraph()
        p.paragraph_format.space_after = Pt(6)
        run = p.add_run(text)
        set_run_font(run, 10, color=BODY)

    if spec.get("servers"):
        h = doc.add_paragraph()
        h.paragraph_format.space_before = Pt(12)
        run = h.add_run("Base URLs")
        set_run_font(run, 12, bold=True, color=INK)
        for server in spec["servers"]:
            p = doc.add_paragraph()
            p.paragraph_format.left_indent = Pt(12)
            p.paragraph_format.space_after = Pt(0)
            run = p.add_run(server.get("url", "—"))
            set_run_font(run, 10, font_name="Consolas", color=BODY)
            if server.get("description"):
                run = p.add_run(f"  — {server['description']}")
                set_run_font(run, 9, italic=True, color=MUTED)


def add_conventions(doc) -> None:
    h = doc.add_paragraph()
    h.paragraph_format.space_before = Pt(18)
    run = h.add_run("2. Common conventions")
    set_run_font(run, 22, bold=False, color=INK)

    add_kv_table(doc, "Request headers", [
        ("Authorization", "Bearer <token> (required for every endpoint except POST /api/v2/authenticate)"),
        ("Content-Type", "application/json (for POST/PUT bodies)"),
        ("Accept", "application/json (default); problem+json is returned for errors"),
    ])

    add_kv_table(doc, "Error responses", [
        ("400", "Request body is malformed or missing required fields"),
        ("403", "Missing or invalid bearer token, or organizer token used on a participant path"),
        ("404", "Resource not found (challenge ID, etc.)"),
        ("429", "Rate limit exceeded; honor the Retry-After header"),
    ])


def add_kv_table(doc, title: str, rows: list[tuple[str, str]]) -> None:
    h = doc.add_paragraph()
    h.paragraph_format.space_before = Pt(12)
    run = h.add_run(title)
    set_run_font(run, 12, bold=True, color=INK)
    table = doc.add_table(rows=1 + len(rows), cols=2)
    table.autofit = False
    table.columns[0].width = Pt(110)
    table.columns[1].width = Pt(380)
    for cell, header in zip(table.rows[0].cells, ("Key", "Value")):
        cell.text = ""
        for paragraph in cell.paragraphs:
            paragraph.paragraph_format.space_after = Pt(0)
        run = cell.paragraphs[0].add_run(header)
        set_run_font(run, 9, bold=True, color=RGBColor(0xFF, 0xFF, 0xFF))
        set_cell_shading(cell, "0F172A")
    for idx, (k, v) in enumerate(rows, start=1):
        cells = table.rows[idx].cells
        write_text(cells[0], k, size_pt=9, color=INK, bold=True)
        write_text(cells[1], v, size_pt=9.5, color=BODY)
        if idx % 2 == 0:
            for cell in cells:
                set_cell_shading(cell, ROW_ALT)
    style_table_borders(table, "E2E8F0")


def add_example_block(doc, lines: list[str]) -> None:
    p = doc.add_paragraph()
    p.paragraph_format.space_before = Pt(6)
    p.paragraph_format.space_after = Pt(6)
    set_paragraph_shading(p, CODE_BG)
    for i, line in enumerate(lines):
        run = p.add_run(line)
        set_run_font(run, 9, font_name="Consolas", color=INK)
        if i < len(lines) - 1:
            run.add_break()


def add_endpoint(doc, path: str, method: str, op: dict) -> None:
    rule = doc.add_paragraph()
    rule.paragraph_format.space_before = Pt(18)
    add_horizontal_line(rule)

    h = doc.add_paragraph()
    h.paragraph_format.space_before = Pt(6)
    run = h.add_run(f"{method.upper()}  {path}")
    set_run_font(run, 13, bold=True, color=INK)

    if op.get("summary"):
        sub = doc.add_paragraph()
        run = sub.add_run(op["summary"])
        set_run_font(run, 10, italic=True, color=MUTED)
    if op.get("description") and op["description"] != op.get("summary"):
        d = doc.add_paragraph()
        d.paragraph_format.space_after = Pt(4)
        run = d.add_run(op["description"])
        set_run_font(run, 10, color=BODY)

    add_kv_table(doc, "Request", [
        ("Authentication", auth_required_for_path(path)),
        ("Method", method.upper()),
        ("Operation ID", op.get("operationId", "—")),
    ])

    params = op.get("parameters", [])
    if params:
        add_param_table(doc, "Parameters", params)
    else:
        h = doc.add_paragraph()
        h.paragraph_format.space_before = Pt(6)
        run = h.add_run("Parameters")
        set_run_font(run, 10, bold=True, color=MUTED)
        p = doc.add_paragraph()
        p.paragraph_format.left_indent = Pt(12)
        run = p.add_run("(none)")
        set_run_font(run, 9.5, italic=True, color=SOFT)

    if "requestBody" in op:
        add_body_table(doc, "Request body", op["requestBody"])
    if "responses" in op:
        add_response_table(doc, "Responses", op["responses"])


def add_param_table(doc, title: str, params: list[dict]) -> None:
    h = doc.add_paragraph()
    h.paragraph_format.space_before = Pt(6)
    run = h.add_run(title)
    set_run_font(run, 10, bold=True, color=MUTED)
    table = doc.add_table(rows=1 + len(params), cols=4)
    table.autofit = False
    table.columns[0].width = Pt(100)
    table.columns[1].width = Pt(50)
    table.columns[2].width = Pt(55)
    table.columns[3].width = Pt(285)
    for cell, header in zip(table.rows[0].cells, ("Name", "In", "Type", "Description")):
        cell.text = ""
        run = cell.paragraphs[0].add_run(header)
        set_run_font(run, 9, bold=True, color=RGBColor(0xFF, 0xFF, 0xFF))
        set_cell_shading(cell, "0F172A")
    for idx, p in enumerate(params, start=1):
        cells = table.rows[idx].cells
        write_text(cells[0], p.get("name", ""), size_pt=9, color=INK, bold=True)
        write_text(cells[1], p.get("in", ""), size_pt=9, color=MUTED)
        write_text(cells[2], short_type(str(p.get("schema", {}).get("type", "—"))), size_pt=9, color=BODY, bold=True)
        write_text(cells[3], p.get("description") or "—", size_pt=9.5, color=BODY)
        if idx % 2 == 0:
            for cell in cells:
                set_cell_shading(cell, ROW_ALT)
    style_table_borders(table, "E2E8F0")


def add_body_table(doc, title: str, body: dict) -> None:
    h = doc.add_paragraph()
    h.paragraph_format.space_before = Pt(6)
    run = h.add_run(title)
    set_run_font(run, 10, bold=True, color=MUTED)
    schema = body.get("schema", {})
    properties = schema.get("properties", {})
    required = set(schema.get("required", []))
    if not properties:
        p = doc.add_paragraph()
        p.paragraph_format.left_indent = Pt(12)
        run = p.add_run("(empty body)")
        set_run_font(run, 9.5, italic=True, color=SOFT)
        return
    table = doc.add_table(rows=1 + len(properties), cols=4)
    table.autofit = False
    table.columns[0].width = Pt(110)
    table.columns[1].width = Pt(50)
    table.columns[2].width = Pt(50)
    table.columns[3].width = Pt(280)
    for cell, header in zip(table.rows[0].cells, ("Field", "Type", "Required", "Description")):
        cell.text = ""
        run = cell.paragraphs[0].add_run(header)
        set_run_font(run, 9, bold=True, color=RGBColor(0xFF, 0xFF, 0xFF))
        set_cell_shading(cell, "0F172A")
    for idx, (name, prop) in enumerate(properties.items(), start=1):
        cells = table.rows[idx].cells
        write_text(cells[0], name, size_pt=9, color=INK, bold=True)
        prop_type = prop.get("type", "")
        if prop_type == "array":
            prop_type = f"[]{short_type(prop.get('items', {}).get('type', '?'))}"
        write_text(cells[1], short_type(prop_type), size_pt=9, color=BODY, bold=True)
        write_text(cells[2], "yes" if name in required else "no",
                   size_pt=9, color=WARN_4XX if name in required else MUTED, bold=True)
        write_text(cells[3], prop.get("description") or "—", size_pt=9.5, color=BODY)
        if idx % 2 == 0:
            for cell in cells:
                set_cell_shading(cell, ROW_ALT)
    style_table_borders(table, "E2E8F0")


def add_response_table(doc, title: str, responses: dict) -> None:
    h = doc.add_paragraph()
    h.paragraph_format.space_before = Pt(6)
    run = h.add_run(title)
    set_run_font(run, 10, bold=True, color=MUTED)
    table = doc.add_table(rows=1 + len(responses), cols=2)
    table.autofit = False
    table.columns[0].width = Pt(60)
    table.columns[1].width = Pt(430)
    for cell, header in zip(table.rows[0].cells, ("Status", "Meaning")):
        cell.text = ""
        run = cell.paragraphs[0].add_run(header)
        set_run_font(run, 9, bold=True, color=RGBColor(0xFF, 0xFF, 0xFF))
        set_cell_shading(cell, "0F172A")
    for idx, (status, body) in enumerate(responses.items(), start=1):
        cells = table.rows[idx].cells
        bg = STATUS_COLOR.get(str(status), WARN_4XX)
        write_text(cells[0], str(status), size_pt=10, bold=True, color=RGBColor(0xFF, 0xFF, 0xFF))
        set_cell_shading(cells[0], bg)
        write_text(cells[1], body.get("description") or "—", size_pt=9.5, color=BODY)
        if idx % 2 == 0:
            set_cell_shading(cells[1], ROW_ALT)
    style_table_borders(table, "E2E8F0")


def add_schema_reference(doc, components: dict) -> None:
    doc.add_page_break()
    h = doc.add_paragraph()
    run = h.add_run("4. Schema reference")
    set_run_font(run, 22, bold=False, color=INK)

    interesting = [
        "submitRequest", "submitResult", "ParticipantSession",
        "Challenge", "ServiceState", "scoreRow", "scoreServiceRow",
        "attackEvent", "GameStatus", "GameMatchStatus",
        "meWireguardConfig", "ProblemDetails", "AuthenticateRequest",
        "AuthenticateResponse", "UnlockData", "SSHData", "FactoryResetData",
    ]
    intro = doc.add_paragraph()
    intro.paragraph_format.space_after = Pt(8)
    run = intro.add_run(
        "Component schemas referenced by the endpoints above. Required fields are "
        "marked with "
    )
    set_run_font(run, 10, color=BODY)
    run = intro.add_run("yes")
    set_run_font(run, 10, bold=True, color=WARN_4XX)
    run = intro.add_run(" in the Required column.")
    set_run_font(run, 10, color=BODY)

    for schema_name in interesting:
        if schema_name not in components:
            continue
        schema = components[schema_name]
        sh = doc.add_paragraph()
        sh.paragraph_format.space_before = Pt(14)
        run = sh.add_run(schema_name)
        set_run_font(run, 12, bold=True, color=INK)

        properties = schema.get("properties", {})
        required = set(schema.get("required", []))
        if not properties:
            p = doc.add_paragraph()
            p.paragraph_format.left_indent = Pt(12)
            run = p.add_run("(no properties)")
            set_run_font(run, 9.5, italic=True, color=SOFT)
            continue
        table = doc.add_table(rows=1 + len(properties), cols=2)
        table.autofit = False
        table.columns[0].width = Pt(160)
        table.columns[1].width = Pt(330)
        for cell, header in zip(table.rows[0].cells, ("Field", "Type")):
            cell.text = ""
            run = cell.paragraphs[0].add_run(header)
            set_run_font(run, 9, bold=True, color=RGBColor(0xFF, 0xFF, 0xFF))
            set_cell_shading(cell, "0F172A")
        for idx, (name, prop) in enumerate(properties.items(), start=1):
            cells = table.rows[idx].cells
            label = f"{name}*" if name in required else name
            write_text(cells[0], label, size_pt=9, color=INK, bold=True)
            if name in required:
                set_cell_shading(cells[0], "FEF3C7")  # amber-100
            prop_type = prop.get("type", "")
            if prop_type == "array":
                prop_type = f"[]{short_type(prop.get('items', {}).get('type', '?'))}"
            elif "$ref" in prop:
                prop_type = prop["$ref"].rsplit("/", 1)[-1]
            write_text(cells[1], short_type(prop_type), size_pt=9, color=BODY)
            if idx % 2 == 0 and name not in required:
                for cell in cells:
                    set_cell_shading(cell, ROW_ALT)
        style_table_borders(table, "E2E8F0")


def add_page_numbers(doc) -> None:
    for section in doc.sections:
        footer = section.footer
        p = footer.paragraphs[0]
        p.alignment = WD_ALIGN_PARAGRAPH.CENTER
        run = p.add_run("Page ")
        set_run_font(run, 9, color=MUTED)
        run = p.add_run()
        fld_char1 = OxmlElement("w:fldChar")
        fld_char1.set(qn("w:fldCharType"), "begin")
        instr = OxmlElement("w:instrText")
        instr.text = " PAGE "
        fld_char2 = OxmlElement("w:fldChar")
        fld_char2.set(qn("w:fldCharType"), "end")
        run._r.append(fld_char1)
        run._r.append(instr)
        run._r.append(fld_char2)
        set_run_font(run, 9, color=MUTED)
        run = p.add_run(" of ")
        set_run_font(run, 9, color=MUTED)
        run = p.add_run()
        fld_char1 = OxmlElement("w:fldChar")
        fld_char1.set(qn("w:fldCharType"), "begin")
        instr = OxmlElement("w:instrText")
        instr.text = " NUMPAGES "
        fld_char2 = OxmlElement("w:fldChar")
        fld_char2.set(qn("w:fldCharType"), "end")
        run._r.append(fld_char1)
        run._r.append(instr)
        run._r.append(fld_char2)
        set_run_font(run, 9, color=MUTED)


def main() -> int:
    with SPEC_PATH.open() as f:
        spec = yaml.safe_load(f)

    doc = Document()
    style = doc.styles["Normal"]
    style.font.name = FONT
    style.font.size = Pt(10)

    info = spec.get("info", {})
    paths = spec.get("paths", {})

    add_cover_page(doc, info)
    add_toc(doc, paths)
    add_overview(doc, spec)
    add_conventions(doc)

    doc.add_page_break()
    h = doc.add_paragraph()
    run = h.add_run("3. Endpoints")
    set_run_font(run, 22, bold=False, color=INK)

    method_order = {"get": 0, "post": 1, "put": 2, "patch": 3, "delete": 4}
    grouped: dict[str, list[tuple[str, str, dict]]] = {}
    for path in sorted(paths.keys()):
        if not is_participant(path):
            continue
        for method in sorted(paths[path].keys(), key=lambda m: method_order.get(m, 99)):
            op = paths[path][method]
            if not isinstance(op, dict) or "responses" not in op:
                continue
            grouped.setdefault(category_for_path(path), []).append((path, method, op))

    for category in ["auth", "me", "challenges", "services", "team", "scoreboard", "submit"]:
        if category not in grouped:
            continue
        ch = doc.add_paragraph()
        ch.paragraph_format.space_before = Pt(18)
        run = ch.add_run(CATEGORY_TITLES[category])
        set_run_font(run, 16, bold=True, color=INK)
        underline = doc.add_paragraph()
        add_horizontal_line(underline, "CBD5E1")

        for path, method, op in grouped[category]:
            add_endpoint(doc, path, method, op)

    add_schema_reference(doc, spec.get("components", {}).get("schemas", {}))

    add_page_numbers(doc)

    doc.save(OUT_PATH)
    print(f"wrote {OUT_PATH}  ({OUT_PATH.stat().st_size:,} bytes)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

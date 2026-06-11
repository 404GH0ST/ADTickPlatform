#!/usr/bin/env python3
"""Generate a participant-facing API reference (docx) from the OpenAPI spec.

Output: docs/participant-api-reference.docx

Design:
  - Calibri family throughout (renders consistently in Word, LibreOffice,
    Pages, Google Docs). Consolas untuk example payloads.
  - Slate-gray + warm amber palette. Tidak ada biru di mana pun.
  - Semua teks UI dihasilkan dalam Bahasa Indonesia. Description
    dari OpenAPI spec di-translate via dict (jika key phrase dikenal) atau
    dibiarkan apa adanya untuk frasa yang sangat teknis.
  - Cover page + TOC + section dividers + page numbers.
  - JSON body examples (request + response) di-extract dari spec dan
    di-render sebagai code block monospace dengan background shading.
  - Per-endpoint block: tabel ringkasan, tabel parameter (path + query),
    tabel body schema, tabel response, dan blok example (jika ada).

Re-run setelah edit docs/platform-api-v2.openapi.yaml:
  python3 scripts/generate-participant-api-doc.py
"""

import json
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

INK = RGBColor(0x0F, 0x17, 0x2A)
BODY = RGBColor(0x1F, 0x29, 0x37)
MUTED = RGBColor(0x64, 0x74, 0x8B)
SOFT = RGBColor(0x94, 0xA3, 0xB8)
OK_2XX = RGBColor(0x04, 0x78, 0x57)
WARN_4XX = RGBColor(0xB4, 0x53, 0x09)
ERR_5XX = RGBColor(0xB9, 0x1C, 0x1C)
OK_2XX_HEX = "047857"
WARN_4XX_HEX = "B45309"
ERR_5XX_HEX = "B91C1C"
ROW_ALT = "F1F5F9"
CODE_BG = "F8FAFC"
REQUIRED_BG = "FEF3C7"
WHITE = RGBColor(0xFF, 0xFF, 0xFF)

FONT = "Calibri"

STATUS_COLOR = {
    "200": OK_2XX_HEX, "201": OK_2XX_HEX, "202": OK_2XX_HEX, "204": OK_2XX_HEX,
    "400": WARN_4XX_HEX, "401": WARN_4XX_HEX, "403": WARN_4XX_HEX, "404": WARN_4XX_HEX, "409": WARN_4XX_HEX, "429": WARN_4XX_HEX,
    "500": ERR_5XX_HEX, "502": ERR_5XX_HEX, "503": ERR_5XX_HEX,
}

TRANSLATIONS = {
    "Authenticates the user and returns a JWT bearer token.":
        "Autentikasi user dan mengembalikan JWT bearer token.",
    "Owning team id; organizer tokens report 0.":
        "ID tim pemilik; token organizer melaporkan 0.",
    "Whether the match is currently in the given state.":
        "Apakah match sedang dalam state tersebut.",
    "Whether the match is accepting submissions.":
        "Apakah match sedang menerima submit.",
    "Whether the game-core reports a failure.":
        "Apakah game-core melaporkan kegagalan.",
    "Whether the most recent scoreboard recompute succeeded.":
        "Apakah scoreboard recompute terakhir berhasil.",
    "Whether a scoreboard recompute is currently in flight.":
        "Apakah scoreboard recompute sedang berjalan.",
    "Whether an async scoreboard recompute is pending.":
        "Apakah ada scoreboard recompute async yang tertunda.",
    "Total async scoreboard recompute failures.":
        "Total kegagalan scoreboard recompute async.",
    "Last successful async scoreboard recompute as a Unix timestamp.":
        "Scoreboard recompute async terakhir yang berhasil, Unix timestamp.",
    "Last failed async scoreboard recompute as a Unix timestamp.":
        "Scoreboard recompute async terakhir yang gagal, Unix timestamp.",
    "Match start time as a Unix timestamp.":
        "Waktu mulai match, Unix timestamp.",
    "Match end time as a Unix timestamp.":
        "Waktu selesai match, Unix timestamp.",
    "Current tick identifier.":
        "Identifier tick saat ini.",
    "Current tick status.":
        "Status tick saat ini.",
    "Whether the current tick is still running.":
        "Apakah tick saat ini masih berjalan.",
    "Whether the scheduler is running.":
        "Apakah scheduler sedang berjalan.",
    "Configured scheduler interval in seconds.":
        "Interval scheduler yang dikonfigurasi (detik).",
    "Most recent tick completed by the scheduler.":
        "Tick terakhir yang diselesaikan scheduler.",
    "Last scheduler run time as a Unix timestamp.":
        "Waktu run scheduler terakhir, Unix timestamp.",
    "Next scheduled run time as a Unix timestamp.":
        "Waktu run terjadwal berikutnya, Unix timestamp.",
    "Whether the requested resource exists.":
        "Apakah resource yang diminta ada.",
    "Whether the team is currently allowed to call this endpoint.":
        "Apakah tim saat ini diizinkan memanggil endpoint ini.",
    "Whether the team is currently paused (cannot submit or unlock).":
        "Apakah tim sedang pause (tidak bisa submit atau unlock).",
    "Whether the team is currently rate-limited.":
        "Apakah tim sedang di-rate-limit.",
    "Game status. See /api/v2/game/status for the full state machine.":
        "Status game. Lihat /api/v2/game/status untuk state machine lengkap.",
    "Whether the requesting player is allowed to operate on this service.":
        "Apakah player yang meminta diizinkan mengoperasikan service ini.",
    "Service output as returned by the checker (only for get/check phases).":
        "Output service seperti dilaporkan checker (hanya untuk fase get/check).",
    "Whether the service is currently unlocked for the team.":
        "Apakah service saat ini sudah di-unlock untuk tim.",
    "Whether the SSH session is currently active.":
        "Apakah sesi SSH saat ini aktif.",
    "Whether the factory reset is currently in progress.":
        "Apakah factory reset sedang berjalan.",
    "Whether the service is being restarted.":
        "Apakah service sedang di-restart.",
    "JSON Patch operations to apply to the source bundle.":
        "Operasi JSON Patch yang diterapkan ke source bundle.",
}


def translate(text: str | None) -> str:
    if not text:
        return ""
    text = text.strip()
    if text in TRANSLATIONS:
        return TRANSLATIONS[text]
    for src, dst in TRANSLATIONS.items():
        if src in text:
            return text.replace(src, dst)
    return text


def is_participant(path: str) -> bool:
    return path.startswith("/api/v2/") and not any(path.startswith(p) for p in EXCLUDE_PATH_PREFIXES)


def short_type(t: str) -> str:
    return {"string": "str", "integer": "int", "boolean": "bool", "number": "num", "array": "[]"}.get(t, t)


def auth_required_for_path(path: str) -> str:
    return "Tidak" if path == "/api/v2/authenticate" else "Bearer <token> (wajib)"


def category_for_path(path: str) -> str:
    if path in ("/api/v2/authenticate", "/api/v2/session"):
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
    "auth": "Autentikasi",
    "me": "Layanan Mandiri",
    "challenges": "Challenge",
    "services": "Service",
    "team": "Tampilan Tim",
    "scoreboard": "Scoreboard & Riwayat Serangan",
    "submit": "Submit Flag",
}


def set_cell_shading(cell, hex_color: str) -> None:
    tc_pr = cell._tc.get_or_add_tcPr()
    shd = OxmlElement("w:shd")
    shd.set(qn("w:val"), "clear")
    shd.set(qn("w:color"), "auto")
    shd.set(qn("w:fill"), hex_color)
    tc_pr.append(shd)


def set_paragraph_shading(paragraph, hex_color: str) -> None:
    p_pr = paragraph._p.get_or_add_pPr()
    shd = OxmlElement("w:shd")
    shd.set(qn("w:val"), "clear")
    shd.set(qn("w:color"), "auto")
    shd.set(qn("w:fill"), hex_color)
    p_pr.append(shd)


def add_horizontal_line(paragraph, hex_color: str = "CBD5E1", size: int = 6) -> None:
    p_pr = paragraph._p.get_or_add_pPr()
    p_bdr = OxmlElement("w:pBdr")
    bottom = OxmlElement("w:bottom")
    bottom.set(qn("w:val"), "single")
    bottom.set(qn("w:sz"), str(size))
    bottom.set(qn("w:space"), "1")
    bottom.set(qn("w:color"), hex_color)
    p_bdr.append(bottom)
    p_pr.append(p_bdr)


def set_run_font(run, size_pt, *, bold=False, italic=False, color=None, font_name=FONT):
    run.font.name = font_name
    run.font.size = Pt(size_pt)
    run.font.bold = bold
    run.font.italic = italic
    if color is not None:
        run.font.color.rgb = color


def style_table_borders(table, hex_color: str = "E2E8F0") -> None:
    for row in table.rows:
        for cell in row.cells:
            tc_pr = cell._tc.get_or_add_tcPr()
            tc_borders = OxmlElement("w:tcBorders")
            for edge in ("top", "left", "bottom", "right"):
                b = OxmlElement(f"w:{edge}")
                b.set(qn("w:val"), "single")
                b.set(qn("w:sz"), "4")
                b.set(qn("w:color"), hex_color)
                tc_borders.append(b)
            tc_pr.append(tc_borders)


def write_text(cell, text, *, size_pt=10, bold=False, italic=False, color=BODY, font_name=FONT):
    cell.text = ""
    para = cell.paragraphs[0]
    para.paragraph_format.space_after = Pt(0)
    run = para.add_run(text)
    set_run_font(run, size_pt, bold=bold, italic=italic, color=color, font_name=font_name)


def write_header_cell(cell, text, hex_bg="0F172A", color=WHITE, size_pt=9):
    cell.text = ""
    para = cell.paragraphs[0]
    para.paragraph_format.space_after = Pt(0)
    run = para.add_run(text)
    set_run_font(run, size_pt, bold=True, color=color)
    set_cell_shading(cell, hex_bg)


def add_kv_table(doc, title, rows, col0_width=120, col1_width=370):
    h = doc.add_paragraph()
    h.paragraph_format.space_before = Pt(12)
    run = h.add_run(title)
    set_run_font(run, 11, bold=True, color=INK)
    table = doc.add_table(rows=1 + len(rows), cols=2)
    table.autofit = False
    table.columns[0].width = Pt(col0_width)
    table.columns[1].width = Pt(col1_width)
    for cell, header in zip(table.rows[0].cells, ("Kunci", "Nilai")):
        write_header_cell(cell, header)
    for idx, (k, v) in enumerate(rows, start=1):
        cells = table.rows[idx].cells
        write_text(cells[0], k, size_pt=9, color=INK, bold=True)
        write_text(cells[1], v, size_pt=9.5, color=BODY)
        if idx % 2 == 0:
            for cell in cells:
                set_cell_shading(cell, ROW_ALT)
    style_table_borders(table)


def add_code_block(doc, lines, *, label=None) -> None:
    if label:
        lp = doc.add_paragraph()
        lp.paragraph_format.space_before = Pt(4)
        run = lp.add_run(label)
        set_run_font(run, 8.5, italic=True, color=MUTED)
    p = doc.add_paragraph()
    p.paragraph_format.space_before = Pt(0)
    p.paragraph_format.space_after = Pt(6)
    p.paragraph_format.left_indent = Pt(6)
    set_paragraph_shading(p, CODE_BG)
    for i, line in enumerate(lines):
        run = p.add_run(line)
        set_run_font(run, 8.5, font_name="Consolas", color=INK)
        if i < len(lines) - 1:
            run.add_break()


def add_endpoint(doc, path, method, op) -> None:
    rule = doc.add_paragraph()
    rule.paragraph_format.space_before = Pt(20)
    add_horizontal_line(rule)

    h = doc.add_paragraph()
    h.paragraph_format.space_before = Pt(6)
    run = h.add_run(f"{method.upper()}  {path}")
    set_run_font(run, 13, bold=True, color=INK)

    if op.get("summary"):
        sub = doc.add_paragraph()
        sub.paragraph_format.space_after = Pt(2)
        run = sub.add_run(translate(op["summary"]))
        set_run_font(run, 10, italic=True, color=MUTED)
    if op.get("description") and op["description"] != op.get("summary"):
        d = doc.add_paragraph()
        d.paragraph_format.space_after = Pt(4)
        run = d.add_run(translate(op["description"]))
        set_run_font(run, 10, color=BODY)

    add_kv_table(doc, "Permintaan", [
        ("Autentikasi", auth_required_for_path(path)),
        ("Metode", method.upper()),
        ("ID Operasi", op.get("operationId", "—")),
    ])

    params = op.get("parameters", [])
    if params:
        add_param_table(doc, "Parameter", params)
    else:
        h = doc.add_paragraph()
        h.paragraph_format.space_before = Pt(6)
        run = h.add_run("Parameter")
        set_run_font(run, 10, bold=True, color=MUTED)
        p = doc.add_paragraph()
        p.paragraph_format.left_indent = Pt(12)
        run = p.add_run("(tidak ada)")
        set_run_font(run, 9.5, italic=True, color=SOFT)

    if "requestBody" in op:
        add_body_table(doc, "Body Permintaan", op["requestBody"])
        render_examples(doc, op["requestBody"].get("content", {}), "Contoh body permintaan")

    if "responses" in op:
        add_response_table(doc, "Respons", op["responses"])
        first_2xx = next((s for s in op["responses"] if s.startswith("2")), None)
        if first_2xx:
            render_examples(
                doc,
                op["responses"][first_2xx].get("content", {}),
                f"Contoh respons {first_2xx}",
            )


def add_param_table(doc, title, params):
    h = doc.add_paragraph()
    h.paragraph_format.space_before = Pt(6)
    run = h.add_run(title)
    set_run_font(run, 10, bold=True, color=MUTED)
    table = doc.add_table(rows=1 + len(params), cols=4)
    table.autofit = False
    table.columns[0].width = Pt(95)
    table.columns[1].width = Pt(50)
    table.columns[2].width = Pt(55)
    table.columns[3].width = Pt(290)
    for cell, header in zip(table.rows[0].cells, ("Nama", "Di", "Tipe", "Deskripsi")):
        write_header_cell(cell, header)
    for idx, p in enumerate(params, start=1):
        cells = table.rows[idx].cells
        write_text(cells[0], p.get("name", ""), size_pt=9, color=INK, bold=True)
        write_text(cells[1], p.get("in", ""), size_pt=9, color=MUTED)
        write_text(cells[2], short_type(str(p.get("schema", {}).get("type", "—"))), size_pt=9, color=BODY, bold=True)
        write_text(cells[3], translate(p.get("description")) or "—", size_pt=9.5, color=BODY)
        if idx % 2 == 0:
            for cell in cells:
                set_cell_shading(cell, ROW_ALT)
    style_table_borders(table)


def add_body_table(doc, title, body):
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
        run = p.add_run("(body kosong)")
        set_run_font(run, 9.5, italic=True, color=SOFT)
        return
    table = doc.add_table(rows=1 + len(properties), cols=4)
    table.autofit = False
    table.columns[0].width = Pt(105)
    table.columns[1].width = Pt(50)
    table.columns[2].width = Pt(50)
    table.columns[3].width = Pt(285)
    for cell, header in zip(table.rows[0].cells, ("Field", "Tipe", "Wajib", "Deskripsi")):
        write_header_cell(cell, header)
    for idx, (name, prop) in enumerate(properties.items(), start=1):
        cells = table.rows[idx].cells
        is_required = name in required
        write_text(cells[0], name + (" *" if is_required else ""), size_pt=9, color=INK, bold=True)
        if is_required:
            set_cell_shading(cells[0], REQUIRED_BG)
        prop_type = prop.get("type", "")
        if prop_type == "array":
            prop_type = f"[]{short_type(prop.get('items', {}).get('type', '?'))}"
        write_text(cells[1], short_type(prop_type), size_pt=9, color=BODY, bold=True)
        write_text(cells[2], "ya" if is_required else "tidak",
                   size_pt=9, color=WARN_4XX if is_required else MUTED, bold=True)
        write_text(cells[3], translate(prop.get("description")) or "—", size_pt=9.5, color=BODY)
        if idx % 2 == 0 and not is_required:
            for cell in cells:
                set_cell_shading(cell, ROW_ALT)
    style_table_borders(table)


def add_response_table(doc, title, responses):
    h = doc.add_paragraph()
    h.paragraph_format.space_before = Pt(6)
    run = h.add_run(title)
    set_run_font(run, 10, bold=True, color=MUTED)
    table = doc.add_table(rows=1 + len(responses), cols=2)
    table.autofit = False
    table.columns[0].width = Pt(60)
    table.columns[1].width = Pt(430)
    for cell, header in zip(table.rows[0].cells, ("Status", "Artinya")):
        write_header_cell(cell, header)
    for idx, (status, body) in enumerate(responses.items(), start=1):
        cells = table.rows[idx].cells
        bg = STATUS_COLOR.get(str(status), WARN_4XX_HEX)
        write_text(cells[0], str(status), size_pt=10, bold=True, color=WHITE)
        set_cell_shading(cells[0], bg)
        write_text(cells[1], translate(body.get("description")) or "—", size_pt=9.5, color=BODY)
        if idx % 2 == 0:
            set_cell_shading(cells[1], ROW_ALT)
    style_table_borders(table)


def render_examples(doc, content, label) -> None:
    json_content = content.get("application/json", {})
    examples = json_content.get("examples", {})
    if not examples:
        return
    for name, example in examples.items():
        value = example.get("value")
        if value is None:
            continue
        pretty = json.dumps(value, indent=2, ensure_ascii=False, default=str)
        lines = [f"// {name}", pretty]
        add_code_block(doc, lines, label=label)


def add_cover_page(doc, info) -> None:
    for _ in range(4):
        doc.add_paragraph()
    title = doc.add_paragraph()
    title.alignment = WD_ALIGN_PARAGRAPH.LEFT
    run = title.add_run("ADTickPlatform")
    set_run_font(run, 32, color=INK)

    sub = doc.add_paragraph()
    sub.alignment = WD_ALIGN_PARAGRAPH.LEFT
    run = sub.add_run("Referensi API Peserta")
    set_run_font(run, 20, color=MUTED)

    rule = doc.add_paragraph()
    add_horizontal_line(rule, "94A3B8")

    meta = doc.add_paragraph()
    meta.paragraph_format.space_before = Pt(18)
    run = meta.add_run(f"Versi {info.get('version', '—')}")
    set_run_font(run, 11, color=BODY)
    meta.add_run("\n")
    run = meta.add_run("Disusun dari docs/platform-api-v2.openapi.yaml")
    set_run_font(run, 11, color=MUTED)
    meta.add_run("\n")
    run = meta.add_run("Regenerate dengan: python3 scripts/generate-participant-api-doc.py")
    set_run_font(run, 10, italic=True, color=SOFT)

    if info.get("description"):
        desc = doc.add_paragraph()
        desc.paragraph_format.space_before = Pt(18)
        run = desc.add_run(translate(info["description"]))
        set_run_font(run, 10, italic=True, color=MUTED)


def add_toc(doc, paths) -> None:
    doc.add_page_break()
    h = doc.add_paragraph()
    run = h.add_run("Daftar Isi")
    set_run_font(run, 20, color=INK)

    sections = [
        ("1.  Gambaran Umum", 12, True),
        ("2.  Konvensi Umum", 8, True),
        ("3.  Endpoint", 12, True),
    ]
    for label, space_before, bold in sections:
        para = doc.add_paragraph()
        para.paragraph_format.space_before = Pt(space_before)
        run = para.add_run(label)
        set_run_font(run, 11, bold=bold, color=INK)

    grouped: dict[str, list[tuple[str, str, str]]] = {}
    for path in sorted(paths.keys()):
        if not is_participant(path):
            continue
        for method in sorted(paths[path].keys()):
            op = paths[path][method]
            if not isinstance(op, dict) or "responses" not in op:
                continue
            grouped.setdefault(category_for_path(path), []).append((method.upper(), path, op.get("summary", "")))

    for category in ["auth", "me", "challenges", "services", "team", "scoreboard", "submit"]:
        if category not in grouped:
            continue
        para = doc.add_paragraph()
        para.paragraph_format.space_before = Pt(4)
        para.paragraph_format.left_indent = Pt(14)
        run = para.add_run(CATEGORY_TITLES[category])
        set_run_font(run, 10, bold=True, color=MUTED)

        for method, path, summary in grouped[category]:
            line = doc.add_paragraph()
            line.paragraph_format.left_indent = Pt(28)
            line.paragraph_format.space_after = Pt(0)
            run = line.add_run(f"{method}  {path}")
            set_run_font(run, 9, color=BODY)
            if summary:
                run = line.add_run(f"  —  {translate(summary)}")
                set_run_font(run, 9, italic=True, color=SOFT)

    para = doc.add_paragraph()
    para.paragraph_format.space_before = Pt(12)
    run = para.add_run("4.  Referensi Skema")
    set_run_font(run, 11, bold=True, color=INK)


def add_overview(doc, spec) -> None:
    doc.add_page_break()
    h = doc.add_paragraph()
    run = h.add_run("1.  Gambaran Umum")
    set_run_font(run, 22, color=INK)

    for text in [
        "Dokumen ini menjelaskan setiap endpoint publik yang dapat dipanggil peserta selama tick "
        "Attack-Defense. Endpoint admin di bawah /api/v2/admin/ dan endpoint internal layanan-ke-layanan "
        "di bawah /internal/v1/ dikecualikan.",
        "Autentikasi menggunakan token bearer JWT dengan masa berlaku 24 jam, diterbitkan oleh "
        "POST /api/v2/authenticate. Sertakan token di setiap permintaan berikutnya sebagai "
        "Authorization: Bearer <token>. Token divalidasi ulang terhadap store pada setiap "
        "permintaan — pemain yang dihapus dari store akan menerima 403 meskipun JWT-nya "
        "masih valid.",
        "Semua endpoint mengembalikan problem+json pada respons non-2xx (RFC 7807). Rate limit "
        "diterapkan per tim untuk sebagian besar endpoint, dengan layer tambahan per-user "
        "pada /api/v2/submit. Melebihi batas akan mengembalikan 429 beserta header Retry-After.",
    ]:
        p = doc.add_paragraph()
        p.paragraph_format.space_after = Pt(6)
        run = p.add_run(text)
        set_run_font(run, 10, color=BODY)

    if spec.get("servers"):
        h = doc.add_paragraph()
        h.paragraph_format.space_before = Pt(12)
        run = h.add_run("URL Dasar")
        set_run_font(run, 12, bold=True, color=INK)
        for server in spec["servers"]:
            p = doc.add_paragraph()
            p.paragraph_format.left_indent = Pt(12)
            p.paragraph_format.space_after = Pt(0)
            run = p.add_run(server.get("url", "—"))
            set_run_font(run, 10, font_name="Consolas", color=BODY)
            if server.get("description"):
                run = p.add_run(f"   —   {server['description']}")
                set_run_font(run, 9, italic=True, color=MUTED)


def add_conventions(doc) -> None:
    h = doc.add_paragraph()
    h.paragraph_format.space_before = Pt(18)
    run = h.add_run("2.  Konvensi Umum")
    set_run_font(run, 22, color=INK)

    add_kv_table(doc, "Header Permintaan", [
        ("Authorization", "Bearer <token> (wajib untuk semua endpoint kecuali POST /api/v2/authenticate)"),
        ("Content-Type", "application/json (untuk body POST/PUT)"),
        ("Accept", "application/json (default); problem+json dikembalikan untuk error"),
    ])

    add_kv_table(doc, "Respons Error Umum", [
        ("400", "Body permintaan malformed atau field wajib hilang"),
        ("403", "Bearer token hilang, tidak valid, expired, atau organizer token dipakai di path peserta"),
        ("404", "Resource tidak ditemukan (challenge ID, dll.)"),
        ("429", "Rate limit terlampaui; hormati header Retry-After"),
    ])


def add_schema_reference(doc, components) -> None:
    doc.add_page_break()
    h = doc.add_paragraph()
    run = h.add_run("4.  Referensi Skema")
    set_run_font(run, 22, color=INK)

    interesante = [
        "submitRequest", "submitResult", "ParticipantSession",
        "Challenge", "ServiceState", "scoreRow", "scoreServiceRow",
        "attackEvent", "GameStatus", "GameMatchStatus",
        "meWireguardConfig", "ProblemDetails", "AuthenticateRequest",
        "AuthenticateResponse", "UnlockData", "SSHData", "FactoryResetData",
    ]
    intro = doc.add_paragraph()
    intro.paragraph_format.space_after = Pt(8)
    run = intro.add_run(
        "Skema komponen yang dirujuk oleh endpoint di atas. Field wajib ditandai "
    )
    set_run_font(run, 10, color=BODY)
    run = intro.add_run("ya")
    set_run_font(run, 10, bold=True, color=WARN_4XX)
    run = intro.add_run(" di kolom Wajib. Skema yang dirujuk sebagai ")
    set_run_font(run, 10, color=BODY)
    run = intro.add_run("$ref")
    set_run_font(run, 10, font_name="Consolas", color=INK)
    run = intro.add_run(" ditampilkan sebagai nama referensinya saja.")
    set_run_font(run, 10, color=BODY)

    for schema_name in interesante:
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
            run = p.add_run("(tidak ada properti)")
            set_run_font(run, 9.5, italic=True, color=SOFT)
            continue
        table = doc.add_table(rows=1 + len(properties), cols=2)
        table.autofit = False
        table.columns[0].width = Pt(170)
        table.columns[1].width = Pt(320)
        for cell, header in zip(table.rows[0].cells, ("Field", "Tipe")):
            write_header_cell(cell, header)
        for idx, (name, prop) in enumerate(properties.items(), start=1):
            cells = table.rows[idx].cells
            is_required = name in required
            label = f"{name}*" if is_required else name
            write_text(cells[0], label, size_pt=9, color=INK, bold=True)
            if is_required:
                set_cell_shading(cells[0], REQUIRED_BG)
            prop_type = prop.get("type", "")
            if prop_type == "array":
                prop_type = f"[]{short_type(prop.get('items', {}).get('type', '?'))}"
            elif "$ref" in prop:
                prop_type = prop["$ref"].rsplit("/", 1)[-1]
            write_text(cells[1], short_type(prop_type), size_pt=9, color=BODY)
            if idx % 2 == 0 and not is_required:
                for cell in cells:
                    set_cell_shading(cell, ROW_ALT)
        style_table_borders(table)


def add_page_numbers(doc) -> None:
    for section in doc.sections:
        footer = section.footer
        p = footer.paragraphs[0]
        p.alignment = WD_ALIGN_PARAGRAPH.CENTER
        run = p.add_run("Halaman ")
        set_run_font(run, 9, color=MUTED)
        for instr_text in (" PAGE ", " NUMPAGES "):
            run = p.add_run()
            fld_begin = OxmlElement("w:fldChar")
            fld_begin.set(qn("w:fldCharType"), "begin")
            instr = OxmlElement("w:instrText")
            instr.text = instr_text
            fld_end = OxmlElement("w:fldChar")
            fld_end.set(qn("w:fldCharType"), "end")
            run._r.append(fld_begin)
            run._r.append(instr)
            run._r.append(fld_end)
            set_run_font(run, 9, color=MUTED)
            if instr_text == " PAGE ":
                run = p.add_run(" dari ")
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
    run = h.add_run("3.  Endpoint")
    set_run_font(run, 22, color=INK)

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

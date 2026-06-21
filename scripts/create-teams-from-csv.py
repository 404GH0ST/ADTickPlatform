#!/usr/bin/env python3
"""Create teams from a CSV file and write team join keys to a text file."""

from __future__ import annotations

import argparse
import csv
import json
import os
import ssl
import sys
import urllib.error
import urllib.request
from pathlib import Path
from typing import Iterable


ROOT_DIR = Path(__file__).resolve().parents[1]
DEFAULT_OUTPUT = "team-keys.txt"
NAME_HEADERS = {"team name", "team_name", "team", "name"}
EMAIL_HEADERS = {"email", "contact_email", "contact email", "team email", "team_email"}


def is_truthy(value: str | None) -> bool:
    return (value or "").strip().lower() in {"1", "true", "yes", "y", "on"}


def is_placeholder_secret(value: str | None) -> bool:
    if not value:
        return True
    return (
        value.startswith("dev-")
        or value == "dev-admin-token"
        or "change-this-" in value
        or "replace-with-" in value
    )


def parse_env_value(value: str) -> str:
    value = value.strip()
    if len(value) >= 2 and value[0] == value[-1] and value[0] in {"'", '"'}:
        return value[1:-1]
    return value


def load_env_file(path: Path, *, override: bool, only_key: str | None = None) -> None:
    if not path.is_file():
        return

    with path.open(encoding="utf-8") as env_file:
        for raw_line in env_file:
            line = raw_line.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            key, value = line.split("=", 1)
            key = key.strip()
            if only_key is not None and key != only_key:
                continue
            if not override and key in os.environ:
                continue
            os.environ[key] = parse_env_value(value)


def load_default_env() -> None:
    env_path = ROOT_DIR / ".env"
    if env_path.is_file():
        load_env_file(env_path, override=False)
    else:
        load_env_file(ROOT_DIR / ".env.example", override=False)

    prod_env = Path(os.environ.get("PROD_ENV", ROOT_DIR / "deploy/compose/prod.env"))
    load_env_file(prod_env, override=True)


def resolve_admin_token(cli_token: str | None) -> str:
    if cli_token:
        return cli_token

    token = os.environ.get("ADMIN_API_TOKEN")
    if is_placeholder_secret(token):
        load_env_file(ROOT_DIR / ".runtime/backend-stack.env", override=True, only_key="ADMIN_API_TOKEN")
        token = os.environ.get("ADMIN_API_TOKEN")

    if is_placeholder_secret(token):
        raise SystemExit(
            "ADMIN_API_TOKEN is required and must not be a placeholder. "
            "Set ADMIN_API_TOKEN or start the local stack so .runtime/backend-stack.env exists."
        )
    return token or ""


def resolve_api_url(cli_url: str | None) -> str:
    api_url = cli_url or os.environ.get("AD_PLATFORM_API_URL", "http://localhost:8080")
    public_url = os.environ.get("AD_PLATFORM_PUBLIC_BASE_URL", "")
    if "api-gateway" in api_url and public_url:
        return public_url.rstrip("/")
    return api_url.rstrip("/")


def ssl_context_for(api_url: str, insecure: bool) -> ssl.SSLContext | None:
    if not api_url.startswith("https://"):
        return None
    if insecure or is_truthy(os.environ.get("ADMIN_CURL_INSECURE")):
        return ssl._create_unverified_context()
    ca_cert = os.environ.get("ADMIN_CA_CERT")
    if ca_cert:
        return ssl.create_default_context(cafile=ca_cert)
    return None


def normalize_header(value: str) -> str:
    return " ".join(value.strip().lower().replace("-", "_").split())


def find_column(headers: Iterable[str], candidates: set[str]) -> str | None:
    for header in headers:
        normalized = normalize_header(header)
        if normalized in candidates or normalized.replace("_", " ") in candidates:
            return header
    return None


def read_teams(csv_path: Path) -> list[tuple[str, str]]:
    with csv_path.open(newline="", encoding="utf-8-sig") as handle:
        rows = list(csv.reader(handle))

    rows = [row for row in rows if any(cell.strip() for cell in row)]
    if not rows:
        raise SystemExit(f"{csv_path} has no team rows.")

    first_row = [cell.strip() for cell in rows[0]]
    normalized_first = {normalize_header(cell) for cell in first_row}
    has_header = bool(
        (normalized_first & NAME_HEADERS or {cell.replace("_", " ") for cell in normalized_first} & NAME_HEADERS)
        and (normalized_first & EMAIL_HEADERS or {cell.replace("_", " ") for cell in normalized_first} & EMAIL_HEADERS)
    )

    teams: list[tuple[str, str]] = []
    if has_header:
        headers = first_row
        name_column = find_column(headers, NAME_HEADERS)
        email_column = find_column(headers, EMAIL_HEADERS)
        if name_column is None or email_column is None:
            raise SystemExit("CSV header must include team name and email columns.")
        for line_number, row in enumerate(rows[1:], start=2):
            record = dict(zip(headers, row))
            name = record.get(name_column, "").strip()
            email = record.get(email_column, "").strip()
            if not name or not email:
                raise SystemExit(f"CSV line {line_number} must include both team name and email.")
            teams.append((name, email))
    else:
        for line_number, row in enumerate(rows, start=1):
            if len(row) < 2:
                raise SystemExit(f"CSV line {line_number} must have at least two columns: team name,email.")
            name = row[0].strip()
            email = row[1].strip()
            if not name or not email:
                raise SystemExit(f"CSV line {line_number} must include both team name and email.")
            teams.append((name, email))

    seen_names: set[str] = set()
    seen_emails: set[str] = set()
    for name, email in teams:
        name_key = name.casefold()
        email_key = email.casefold()
        if name_key in seen_names:
            raise SystemExit(f"duplicate team name in CSV: {name}")
        if email_key in seen_emails:
            raise SystemExit(f"duplicate email in CSV: {email}")
        seen_names.add(name_key)
        seen_emails.add(email_key)

    return teams


def create_team(
    api_url: str,
    admin_token: str,
    ssl_context: ssl.SSLContext | None,
    name: str,
    email: str,
) -> dict[str, object]:
    payload = json.dumps({"name": name, "contact_email": email}).encode()
    request = urllib.request.Request(
        f"{api_url}/api/v2/admin/teams",
        data=payload,
        method="POST",
        headers={
            "Authorization": f"Bearer {admin_token}",
            "Content-Type": "application/json",
            "Accept": "application/json",
        },
    )

    try:
        with urllib.request.urlopen(request, context=ssl_context, timeout=30) as response:
            return json.loads(response.read().decode())
    except urllib.error.HTTPError as error:
        body = error.read().decode(errors="replace")
        raise SystemExit(f"create team {name!r} failed (status={error.code}):\n{body}") from error
    except urllib.error.URLError as error:
        raise SystemExit(f"create team {name!r} failed: {error.reason}") from error


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Create platform teams from a CSV containing team name and email columns.",
    )
    parser.add_argument("csv", type=Path, help="CSV file. Use headers or plain rows: team name,email")
    parser.add_argument("-o", "--output", type=Path, default=Path(DEFAULT_OUTPUT), help=f"output txt file (default: {DEFAULT_OUTPUT})")
    parser.add_argument("--api-url", help="admin API base URL (default: AD_PLATFORM_API_URL or http://localhost:8080)")
    parser.add_argument("--admin-token", help="admin bearer token (default: ADMIN_API_TOKEN)")
    parser.add_argument("--insecure", action="store_true", help="skip HTTPS certificate verification")
    parser.add_argument("--dry-run", action="store_true", help="parse CSV and print teams without calling the API")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    csv_path = args.csv.expanduser()
    if not csv_path.is_file():
        raise SystemExit(f"CSV file not found: {csv_path}")

    load_default_env()
    teams = read_teams(csv_path)

    if args.dry_run:
        print(f"parsed {len(teams)} team(s):")
        for name, email in teams:
            print(f"- {name} <{email}>")
        return 0

    api_url = resolve_api_url(args.api_url)
    admin_token = resolve_admin_token(args.admin_token)
    ssl_context = ssl_context_for(api_url, args.insecure)
    output_path = args.output.expanduser()
    output_path.parent.mkdir(parents=True, exist_ok=True)

    key_lines: list[str] = []
    print(f"creating {len(teams)} team(s) via {api_url}")
    for name, email in teams:
        response = create_team(api_url, admin_token, ssl_context, name, email)
        join_key = str(response.get("join_key") or response.get("team_key") or "").strip()
        if not join_key:
            raise SystemExit(f"create team {name!r} succeeded but response did not include join_key.")
        key_lines.append(f"{name}:{join_key}\n")
        print(f"created {name}")

    output_path.write_text("".join(key_lines), encoding="utf-8")
    output_path.chmod(0o600)
    print(f"wrote {len(key_lines)} team key(s) to {output_path}")
    return 0


if __name__ == "__main__":
    sys.exit(main())

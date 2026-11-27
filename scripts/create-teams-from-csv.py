#!/usr/bin/env python3
"""Create teams from a CSV file and write team join keys to a text file."""

from __future__ import annotations

import argparse
import csv
import json
import os
import ssl
import sys
import tempfile
import urllib.error
import urllib.request
from pathlib import Path
from typing import Iterable


ROOT_DIR = Path(__file__).resolve().parents[1]
DEFAULT_OUTPUT = "team-keys.txt"
NAME_HEADERS = {"team name", "team_name", "team", "name"}
EMAIL_HEADERS = {"email", "contact_email", "contact email", "team email", "team_email"}
INSTITUTION_HEADERS = {"institution", "school", "university", "organization", "organisation", "affiliation"}
# Matches maxTeamInstitutionLength in the API gateway; checked up front so a bad
# row fails the import before any team is created.
MAX_INSTITUTION_LENGTH = 120


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


def normalize_institution(value: str, line_number: int) -> str:
    institution = " ".join(value.split())
    if len(institution) > MAX_INSTITUTION_LENGTH:
        raise SystemExit(f"CSV line {line_number} institution must be at most {MAX_INSTITUTION_LENGTH} characters.")
    return institution


def read_teams(csv_path: Path) -> list[tuple[str, str, str]]:
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

    teams: list[tuple[str, str, str]] = []
    if has_header:
        headers = first_row
        name_column = find_column(headers, NAME_HEADERS)
        email_column = find_column(headers, EMAIL_HEADERS)
        institution_column = find_column(headers, INSTITUTION_HEADERS)
        if name_column is None or email_column is None:
            raise SystemExit("CSV header must include team name and email columns.")
        for line_number, row in enumerate(rows[1:], start=2):
            record = dict(zip(headers, row))
            name = record.get(name_column, "").strip()
            email = record.get(email_column, "").strip()
            if not name or not email:
                raise SystemExit(f"CSV line {line_number} must include both team name and email.")
            institution = normalize_institution(record.get(institution_column, "") if institution_column else "", line_number)
            teams.append((name, email, institution))
    else:
        for line_number, row in enumerate(rows, start=1):
            if len(row) < 2:
                raise SystemExit(f"CSV line {line_number} must have at least two columns: team name,email[,institution].")
            name = row[0].strip()
            email = row[1].strip()
            if not name or not email:
                raise SystemExit(f"CSV line {line_number} must include both team name and email.")
            institution = normalize_institution(row[2] if len(row) > 2 else "", line_number)
            teams.append((name, email, institution))

    seen_names: set[str] = set()
    seen_emails: set[str] = set()
    for name, email, _institution in teams:
        name_key = name.casefold()
        email_key = email.casefold()
        if name_key in seen_names:
            raise SystemExit(f"duplicate team name in CSV: {name}")
        if email_key in seen_emails:
            raise SystemExit(f"duplicate email in CSV: {email}")
        seen_names.add(name_key)
        seen_emails.add(email_key)

    return teams


def admin_request(
    api_url: str,
    admin_token: str,
    ssl_context: ssl.SSLContext | None,
    method: str,
    path: str,
    body: dict[str, str] | None,
    action: str,
) -> object:
    request = urllib.request.Request(
        f"{api_url}{path}",
        data=json.dumps(body).encode() if body is not None else None,
        method=method,
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
        detail = error.read().decode(errors="replace")
        raise SystemExit(f"{action} failed (status={error.code}):\n{detail}") from error
    except urllib.error.URLError as error:
        raise SystemExit(f"{action} failed: {error.reason}") from error


def create_team(
    api_url: str,
    admin_token: str,
    ssl_context: ssl.SSLContext | None,
    name: str,
    email: str,
    institution: str,
) -> dict[str, object]:
    body = {"name": name, "contact_email": email}
    if institution:
        body["institution"] = institution
    response = admin_request(api_url, admin_token, ssl_context, "POST", "/api/v2/admin/teams", body, f"create team {name!r}")
    return response if isinstance(response, dict) else {}


def list_teams(api_url: str, admin_token: str, ssl_context: ssl.SSLContext | None) -> list[dict[str, object]]:
    response = admin_request(api_url, admin_token, ssl_context, "GET", "/api/v2/admin/teams", None, "list teams")
    if not isinstance(response, list):
        raise SystemExit("list teams returned an unexpected response.")
    return [team for team in response if isinstance(team, dict)]


def update_team_institution(
    api_url: str,
    admin_token: str,
    ssl_context: ssl.SSLContext | None,
    team: dict[str, object],
    institution: str,
) -> dict[str, object]:
    # The update endpoint requires name and email; resend the stored values so
    # only the institution changes.
    body = {
        "name": str(team.get("name") or ""),
        "contact_email": str(team.get("contact_email") or ""),
        "institution": institution,
    }
    response = admin_request(
        api_url, admin_token, ssl_context, "PUT", f"/api/v2/admin/teams/{team.get('id')}", body,
        f"update team {body['name']!r}",
    )
    return response if isinstance(response, dict) else {}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Create platform teams from a CSV containing team name and email columns.",
    )
    parser.add_argument("csv", type=Path, help="CSV file. Use headers (team name, email, optional institution) or plain rows: team name,email[,institution]")
    parser.add_argument("-o", "--output", type=Path, default=Path(DEFAULT_OUTPUT), help=f"output txt file (default: {DEFAULT_OUTPUT})")
    parser.add_argument("--api-url", help="admin API base URL (default: AD_PLATFORM_API_URL or http://localhost:8080)")
    parser.add_argument("--admin-token", help="admin bearer token (default: ADMIN_API_TOKEN)")
    parser.add_argument("--insecure", action="store_true", help="skip HTTPS certificate verification")
    parser.add_argument(
        "--update-existing",
        action="store_true",
        help="for teams that already exist (matched by name), set the CSV institution instead of failing; "
        "name, email and join key are left as they are",
    )
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
        for name, email, institution in teams:
            suffix = f" [{institution}]" if institution else ""
            print(f"- {name} <{email}>{suffix}")
        return 0

    api_url = resolve_api_url(args.api_url)
    admin_token = resolve_admin_token(args.admin_token)
    ssl_context = ssl_context_for(api_url, args.insecure)
    output_path = args.output.expanduser()
    output_path.parent.mkdir(parents=True, exist_ok=True)

    existing: dict[str, dict[str, object]] = {}
    if args.update_existing:
        existing_teams = list_teams(api_url, admin_token, ssl_context)
        existing = {str(team.get("name") or "").casefold(): team for team in existing_teams}
        existing_emails = {str(team.get("contact_email") or "").casefold(): team for team in existing_teams}
        # Check every row before changing anything: a new name whose email
        # belongs to another team would be rejected mid-run.
        for name, email, _institution in teams:
            owner = existing_emails.get(email.casefold())
            if name.casefold() not in existing and owner is not None:
                raise SystemExit(f"email {email} already belongs to team {owner.get('name')!r}, not {name!r}.")

    key_lines: list[str] = []
    print(f"processing {len(teams)} team(s) via {api_url}")
    for name, email, institution in teams:
        current = existing.get(name.casefold())
        if current is not None:
            if str(current.get("institution") or "") == institution:
                response = current
                print(f"unchanged {name}")
            else:
                response = update_team_institution(api_url, admin_token, ssl_context, current, institution)
                print(f"updated {name}")
        else:
            response = create_team(api_url, admin_token, ssl_context, name, email, institution)
            print(f"created {name}")
        join_key = str(response.get("join_key") or response.get("team_key") or current and current.get("join_key") or "").strip()
        if not join_key:
            raise SystemExit(f"team {name!r} has no join_key in the API response.")
        key_lines.append(f"{name}:{join_key}\n")

    with tempfile.NamedTemporaryFile(mode="w", encoding="utf-8", dir=output_path.parent, delete=False) as key_file:
        temporary_path = Path(key_file.name)
        try:
            key_file.writelines(key_lines)
            key_file.close()
            temporary_path.replace(output_path)
        finally:
            temporary_path.unlink(missing_ok=True)
    print(f"wrote {len(key_lines)} team key(s) to {output_path}")
    return 0


if __name__ == "__main__":
    sys.exit(main())

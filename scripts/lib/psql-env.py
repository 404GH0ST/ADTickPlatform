"""Run psql using POSTGRES_DSN without placing the database password in argv."""

import os
import re
import sys
from urllib.parse import unquote, urlsplit


# libpq also accepts keyword/value strings, with optional single quotes and
# backslash escapes. Preserve the spelling of every option other than password.
CONNINFO_OPTION = re.compile(r"""\s*([^\s=]+)\s*=\s*('(?:\\.|[^'\\])*'|(?:\\.|[^\s\\])*)""", re.DOTALL)


def split_conninfo(dsn: str) -> tuple[str, str | None]:
    if dsn.startswith(("postgresql://", "postgres://")):
        uri = urlsplit(dsn, allow_fragments=False)
        userinfo, separator, hosts = uri.netloc.rpartition("@")
        username, has_password, password = userinfo.partition(":")
        password = unquote(password) if has_password else None
        query = []
        for option in uri.query.split("&"):
            key, _, value = option.partition("=")
            if unquote(key) == "password":
                password = unquote(value)
            else:
                query.append(option)
        authority = username + separator + hosts if separator else uri.netloc
        target = f"{uri.scheme}://{authority}{uri.path}"
        if sanitized_query := "&".join(query):
            target += f"?{sanitized_query}"
        return target, password

    if "=" not in dsn:
        return dsn, None

    options = []
    password = None
    remaining = dsn.strip()
    while remaining:
        option = CONNINFO_OPTION.match(remaining)
        if option is None:
            raise ValueError("invalid PostgreSQL connection options")
        key, value = option.groups()
        if key == "password":
            if value.startswith("'") and value.endswith("'"):
                value = value[1:-1]
            password = re.sub(r"\\(.)", r"\1", value, flags=re.DOTALL)
        else:
            options.append(option.group(0))
        remaining = remaining[option.end():].lstrip()
    return " ".join(options), password


if __name__ == "__main__":
    try:
        target, password = split_conninfo(os.environ.pop("POSTGRES_DSN"))
        if password is not None:
            os.environ["PGPASSWORD"] = password
        os.execvp("psql", ["psql", "--dbname", target, *sys.argv[1:]])
    except (KeyError, OSError, ValueError):
        sys.exit("could not start psql with the configured PostgreSQL connection options")

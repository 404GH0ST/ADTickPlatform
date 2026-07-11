export type TeamCsvPlayer = {
  display_name: string;
  email: string;
  password: string;
  role: string;
};

export type TeamCsvRow = {
  name: string;
  contact_email: string;
  players: TeamCsvPlayer[];
};

export function parseTeamCsv(text: string): TeamCsvRow[] {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  if (lines.length === 0) {
    return [];
  }

  const header = splitCsvLine(lines[0]).map((value) => value.toLowerCase());
  const hasHeader =
    header.includes("team_name") || header.includes("contact_email");
  const rows = hasHeader ? lines.slice(1) : lines;
  const index = (name: string) => header.indexOf(name);

  const teams = new Map<string, TeamCsvRow>();

  for (const line of rows) {
    const cells = splitCsvLine(line);
    const teamName = hasHeader
      ? (cells[index("team_name")] ?? "")
      : (cells[0] ?? "");
    const contactEmail = hasHeader
      ? (cells[index("contact_email")] ?? "")
      : (cells[1] ?? "");
    if (!teamName.trim() || !contactEmail.trim()) {
      continue;
    }
    const key = `${teamName.trim().toLowerCase()}|${contactEmail.trim().toLowerCase()}`;
    if (!teams.has(key)) {
      teams.set(key, {
        name: teamName.trim(),
        contact_email: contactEmail.trim(),
        players: [],
      });
    }
    const displayName = hasHeader
      ? (cells[index("player_display_name")] ?? "")
      : (cells[2] ?? "");
    const email = hasHeader
      ? (cells[index("player_email")] ?? "")
      : (cells[3] ?? "");
    const password = hasHeader
      ? (cells[index("player_password")] ?? "")
      : (cells[4] ?? "");
    const role = hasHeader
      ? (cells[index("player_role")] ?? "member")
      : (cells[5] ?? "member");
    if (displayName.trim() && email.trim() && password.trim()) {
      teams.get(key)?.players.push({
        display_name: displayName.trim(),
        email: email.trim(),
        password: password.trim(),
        role: role.trim() || "member",
      });
    }
  }

  return Array.from(teams.values());
}

export function splitCsvLine(line: string): string[] {
  const cells: string[] = [];
  let current = "";
  let inQuotes = false;
  for (let i = 0; i < line.length; i += 1) {
    const char = line[i];
    if (char === '"') {
      if (inQuotes && line[i + 1] === '"') {
        current += '"';
        i += 1;
      } else {
        inQuotes = !inQuotes;
      }
      continue;
    }
    if (char === "," && !inQuotes) {
      cells.push(current.trim());
      current = "";
      continue;
    }
    current += char;
  }
  cells.push(current.trim());
  return cells;
}

export function toCsv(rows: string[][]): string {
  return rows
    .map((row) =>
      row
        .map((cell) => {
          const value = String(cell ?? "");
          if (/[",\n]/.test(value)) {
            return `"${value.replaceAll('"', '""')}"`;
          }
          return value;
        })
        .join(","),
    )
    .join("\n");
}

export function downloadText(
  filename: string,
  content: string,
  mime = "text/csv",
) {
  const blob = new Blob([content], { type: mime });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(url);
}

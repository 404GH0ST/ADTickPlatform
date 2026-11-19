// @ts-expect-error Bun exposes its test module at runtime without bundled TypeScript types.
import { describe, expect, test } from "bun:test";

import { parseTeamCsv, toCsv } from "./team-csv";

describe("parseTeamCsv", () => {
  test("groups records by team and decodes quoted text in reordered columns", () => {
    const csv = [
      "CONTACT_EMAIL,TEAM_NAME,PLAYER_EMAIL,PLAYER_DISPLAY_NAME,PLAYER_PASSWORD,PLAYER_ROLE",
      'team@example.test,"Blue, Inc.",one@example.test,"Alex ""Ace""",test-pass-one,captain',
      'TEAM@EXAMPLE.TEST,"BLUE, INC.",two@example.test,Bailey,test-pass-two,',
      ',Incomplete,,,,',
    ].join("\r\n");
    expect(parseTeamCsv(csv)).toEqual([{
      name: "Blue, Inc.",
      contact_email: "team@example.test",
      players: [
        { display_name: 'Alex "Ace"', email: "one@example.test", password: "test-pass-one", role: "captain" },
        { display_name: "Bailey", email: "two@example.test", password: "test-pass-two", role: "member" },
      ],
    }]);
  });

  test("accepts headerless team-only data and skips blank rows", () => {
    expect(parseTeamCsv(" \n\n")).toEqual([]);
    expect(parseTeamCsv("Team A,a@example.test\n\nTeam B,b@example.test,,partial@example.test")).toEqual([
      { name: "Team A", contact_email: "a@example.test", players: [] },
      { name: "Team B", contact_email: "b@example.test", players: [] },
    ]);
  });
});

describe("toCsv", () => {
  test("neutralizes cells that would start a spreadsheet formula", () => {
    const csv = toCsv([
      ['=HYPERLINK("http://attacker.example/x","Click")'],
      ["@SUM(1+1)"],
      ["+cmd|' /C calc'!A0"],
      ["-2+3"],
      ["  =1+1"],
      ["\t=1+1"],
    ]);
    expect(csv.split("\n")).toEqual([
      `"'=HYPERLINK(""http://attacker.example/x"",""Click"")"`,
      "'@SUM(1+1)",
      "'+cmd|' /C calc'!A0",
      "'-2+3",
      "'  =1+1",
      "'\t=1+1",
    ]);
  });

  test("keeps plain numbers and ordinary text unchanged", () => {
    expect(toCsv([["1", "Team Alpha", "-3", "+12", "4.5", "a=b"]])).toBe(
      "1,Team Alpha,-3,+12,4.5,a=b",
    );
  });

  test("quotes commas, quotes, and line breaks", () => {
    expect(toCsv([['a,b', 'say "hi"', "x\ry"]])).toBe(
      '"a,b","say ""hi""","x\ry"',
    );
  });
});

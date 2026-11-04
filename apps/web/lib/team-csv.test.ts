// @ts-expect-error Bun exposes its test module at runtime without bundled TypeScript types.
import { describe, expect, test } from "bun:test";

import { toCsv } from "./team-csv";

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

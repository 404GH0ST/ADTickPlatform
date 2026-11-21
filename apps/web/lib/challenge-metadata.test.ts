// @ts-expect-error Bun exposes its test module at runtime without bundled TypeScript types.
import { describe, expect, test } from "bun:test";

import {
  challengeCategoryLabel,
  describeConnection,
  parseChallengeCategory,
  parseChallengeProtocol,
} from "./challenge-metadata";

describe("parseChallengeCategory", () => {
  test("accepts the three categories in any case", () => {
    expect(parseChallengeCategory("pwn")).toBe("pwn");
    expect(parseChallengeCategory(" WEB ")).toBe("web");
    expect(parseChallengeCategory("Crypto")).toBe("crypto");
  });

  test("anything else is uncategorized", () => {
    expect(parseChallengeCategory("")).toBe("");
    expect(parseChallengeCategory("misc")).toBe("");
    expect(parseChallengeCategory(undefined)).toBe("");
    expect(parseChallengeCategory(3)).toBe("");
  });

  test("labels spell out web exploitation", () => {
    expect(challengeCategoryLabel("web")).toBe("Web exploitation");
    expect(challengeCategoryLabel("")).toBe("Uncategorized");
  });
});

describe("parseChallengeProtocol", () => {
  test("http is http; everything else, including a missing field, is tcp", () => {
    expect(parseChallengeProtocol("http")).toBe("http");
    expect(parseChallengeProtocol("HTTP")).toBe("http");
    expect(parseChallengeProtocol("tcp")).toBe("tcp");
    expect(parseChallengeProtocol(undefined)).toBe("tcp");
    expect(parseChallengeProtocol("https")).toBe("tcp");
  });
});

describe("describeConnection", () => {
  test("tcp becomes an nc command", () => {
    expect(describeConnection("10.80.4.11:30050", "tcp")).toEqual({
      protocol: "tcp",
      label: "TCP",
      host: "10.80.4.11",
      port: 30050,
      display: "nc 10.80.4.11 30050",
      copyText: "nc 10.80.4.11 30050",
    });
  });

  test("http becomes an openable URL", () => {
    const connection = describeConnection("10.80.4.11:8080", "http");
    expect(connection?.display).toBe("http://10.80.4.11:8080/");
    expect(connection?.href).toBe("http://10.80.4.11:8080/");
    expect(connection?.label).toBe("HTTP");
  });

  test("anything that is not host:port has no connection line", () => {
    expect(describeConnection("", "tcp")).toBeNull();
    expect(describeConnection("10.80.4.11", "tcp")).toBeNull();
    expect(describeConnection("10.80.4.11:0", "http")).toBeNull();
    expect(describeConnection("10.80.4.11:70000", "tcp")).toBeNull();
    expect(describeConnection("http://10.80.4.11:80", "http")).toBeNull();
  });
});

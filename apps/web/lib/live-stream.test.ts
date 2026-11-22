// @ts-expect-error Bun exposes its test module at runtime without bundled TypeScript types.
import { afterEach, expect, test } from "bun:test";

import { LiveStream } from "./live-stream";

class FakeEventSource {
  static readonly CLOSED = 2;
  static created: FakeEventSource[] = [];
  readyState = 1;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: (() => void) | null = null;

  constructor() {
    FakeEventSource.created.push(this);
  }

  close() {
    this.readyState = FakeEventSource.CLOSED;
  }
}

const originalEventSource = globalThis.EventSource;
const wait = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

function useFakeEventSource() {
  FakeEventSource.created = [];
  globalThis.EventSource = FakeEventSource as unknown as typeof EventSource;
}

afterEach(() => {
  globalThis.EventSource = originalEventSource;
});

test("a stream closed by an error response reconnects and keeps its handlers", async () => {
  useFakeEventSource();
  const stream = new LiveStream("/stream", 1);
  const received: string[] = [];
  let errors = 0;
  stream.onmessage = (event) => received.push(event.data);
  stream.onerror = () => { errors += 1; };

  const first = FakeEventSource.created[0];
  first.readyState = FakeEventSource.CLOSED;
  first.onerror?.();
  await wait(10);

  expect(errors).toBe(1);
  expect(FakeEventSource.created).toHaveLength(2);
  FakeEventSource.created[1].onmessage?.({ data: "after recovery" } as MessageEvent);
  expect(received).toEqual(["after recovery"]);
  stream.close();
});

test("a dropped connection the browser is retrying is left alone", async () => {
  useFakeEventSource();
  const stream = new LiveStream("/stream", 1);
  FakeEventSource.created[0].onerror?.();
  await wait(10);
  expect(FakeEventSource.created).toHaveLength(1);
  stream.close();
});

test("closing cancels a pending reconnect", async () => {
  useFakeEventSource();
  const stream = new LiveStream("/stream", 5);
  const first = FakeEventSource.created[0];
  first.readyState = FakeEventSource.CLOSED;
  first.onerror?.();
  stream.close();
  await wait(15);
  expect(FakeEventSource.created).toHaveLength(1);
});

function fakeTabVisibility(protocol: string) {
  const originalEntries = performance.getEntriesByType.bind(performance);
  const originalDocument = (globalThis as { document?: unknown }).document;
  const listeners = new Set<() => void>();
  const page = {
    visibilityState: "visible",
    addEventListener: (_type: string, listener: () => void) => listeners.add(listener),
    removeEventListener: (_type: string, listener: () => void) => listeners.delete(listener),
  };
  (globalThis as { document?: unknown }).document = page;
  performance.getEntriesByType = (() => [{ nextHopProtocol: protocol }]) as unknown as typeof performance.getEntriesByType;
  return {
    listeners,
    setVisibility(state: "visible" | "hidden") {
      page.visibilityState = state;
      for (const listener of listeners) listener();
    },
    restore() {
      performance.getEntriesByType = originalEntries;
      (globalThis as { document?: unknown }).document = originalDocument;
    },
  };
}

test("on HTTP/1.1 a hidden tab frees its connection and reconnects when shown", () => {
  useFakeEventSource();
  const tab = fakeTabVisibility("http/1.1");
  try {
    const stream = new LiveStream("/stream", 1);
    let downs = 0;
    stream.onerror = () => { downs += 1; };
    tab.setVisibility("hidden");
    expect(FakeEventSource.created[0].readyState).toBe(FakeEventSource.CLOSED);
    expect(downs).toBe(1);
    tab.setVisibility("visible");
    expect(FakeEventSource.created).toHaveLength(2);
    stream.close();
    expect(tab.listeners.size).toBe(0);
  } finally {
    tab.restore();
  }
});

test("on HTTP/2 a hidden tab keeps its stream", () => {
  useFakeEventSource();
  const tab = fakeTabVisibility("h2");
  try {
    const stream = new LiveStream("/stream", 1);
    tab.setVisibility("hidden");
    expect(FakeEventSource.created).toHaveLength(1);
    expect(FakeEventSource.created[0].readyState).toBe(1);
    stream.close();
  } finally {
    tab.restore();
  }
});

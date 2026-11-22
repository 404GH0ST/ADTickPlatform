const RETRY_DELAY_MS = 5_000;

/**
 * HTTP/1.1 allows a browser six connections per site, shared by every tab, and
 * each open stream holds one. HTTP/2 has no such limit.
 */
function sharesBrowserConnections(): boolean {
  if (typeof performance === "undefined" || typeof document === "undefined") return false;
  const [navigation] = performance.getEntriesByType("navigation") as PerformanceNavigationTiming[];
  return navigation?.nextHopProtocol?.startsWith("http/1") ?? false;
}

/**
 * An EventSource that comes back after an outage. The browser retries a dropped
 * connection itself, but an error response such as a 502 closes the source for
 * good, which would leave a page labelled "Live" that never updates again.
 *
 * On HTTP/1.1 a hidden tab also lets go of its connection, so a few open tabs
 * can't starve the visible one of requests. It reconnects when shown and the
 * stream's first frame brings it up to date.
 */
export class LiveStream {
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: (() => void) | null = null;

  private readonly url: string;
  private readonly retryDelayMs: number;
  private readonly pausesWhenHidden: boolean;
  private source: EventSource | null = null;
  private retryTimer: ReturnType<typeof setTimeout> | undefined;
  private closed = false;

  constructor(url: string, retryDelayMs = RETRY_DELAY_MS) {
    this.url = url;
    this.retryDelayMs = retryDelayMs;
    this.pausesWhenHidden = sharesBrowserConnections();
    if (this.pausesWhenHidden) {
      document.addEventListener("visibilitychange", this.handleVisibilityChange);
    }
    if (!this.isPaused()) {
      this.source = this.connect();
    }
  }

  private isPaused(): boolean {
    return this.pausesWhenHidden && document.visibilityState === "hidden";
  }

  private readonly handleVisibilityChange = () => {
    if (this.isPaused()) {
      this.disconnect();
      this.onerror?.();
    } else if (!this.source) {
      clearTimeout(this.retryTimer);
      this.source = this.connect();
    }
  };

  private connect(): EventSource {
    const source = new EventSource(this.url);
    source.onmessage = (event) => this.onmessage?.(event);
    source.onerror = () => {
      this.onerror?.();
      if (source.readyState === EventSource.CLOSED && !this.closed && this.source === source) {
        this.source = null;
        clearTimeout(this.retryTimer);
        this.retryTimer = setTimeout(() => {
          if (!this.isPaused()) this.source = this.connect();
        }, this.retryDelayMs);
      }
    };
    return source;
  }

  private disconnect(): void {
    clearTimeout(this.retryTimer);
    this.source?.close();
    this.source = null;
  }

  close(): void {
    this.closed = true;
    if (this.pausesWhenHidden) {
      document.removeEventListener("visibilitychange", this.handleVisibilityChange);
    }
    this.disconnect();
  }
}

const RETRY_DELAY_MS = 5_000;

/**
 * An EventSource that comes back after an outage. The browser retries a dropped
 * connection itself, but an error response such as a 502 closes the source for
 * good, which would leave a page labelled "Live" that never updates again.
 */
export class LiveStream {
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: (() => void) | null = null;

  private readonly url: string;
  private readonly retryDelayMs: number;
  private source: EventSource;
  private retryTimer: ReturnType<typeof setTimeout> | undefined;
  private closed = false;

  constructor(url: string, retryDelayMs = RETRY_DELAY_MS) {
    this.url = url;
    this.retryDelayMs = retryDelayMs;
    this.source = this.connect();
  }

  private connect(): EventSource {
    const source = new EventSource(this.url);
    source.onmessage = (event) => this.onmessage?.(event);
    source.onerror = () => {
      this.onerror?.();
      if (source.readyState === EventSource.CLOSED && !this.closed) {
        this.retryTimer = setTimeout(() => {
          this.source = this.connect();
        }, this.retryDelayMs);
      }
    };
    return source;
  }

  close(): void {
    this.closed = true;
    clearTimeout(this.retryTimer);
    this.source.close();
  }
}

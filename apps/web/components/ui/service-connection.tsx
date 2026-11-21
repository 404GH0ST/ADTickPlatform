"use client";

import { useEffect, useRef, useState, type ReactElement } from "react";
import { Check, Copy, ExternalLink } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type { ServiceConnection } from "@/lib/challenge-metadata";
import { writeTextToClipboard } from "@/lib/clipboard";
import { cn } from "@/lib/utils";

export type CopyStatus = "idle" | "copied" | "failed";

/**
 * Copy text and report the outcome for two seconds. The status is meant for
 * both the button label and a polite live region, so a screen reader hears
 * "Copied" without the focus moving.
 */
export function useCopyFeedback(): { status: CopyStatus; copy: (text: string) => Promise<void> } {
  const [status, setStatus] = useState<CopyStatus>("idle");
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => () => {
    if (timer.current) clearTimeout(timer.current);
  }, []);
  async function copy(text: string) {
    try {
      await writeTextToClipboard(text);
      setStatus("copied");
    } catch {
      setStatus("failed");
    }
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => setStatus("idle"), 2000);
  }
  return { status, copy };
}

export function copyButtonLabel(connection: ServiceConnection, status: CopyStatus): string {
  if (status === "copied") return "Copied";
  if (status === "failed") return "Copy failed";
  return connection.protocol === "http" ? "Copy URL" : "Copy command";
}

/**
 * How to reach a service: the protocol, then a URL (HTTP, openable) or an nc
 * command (TCP), with a copy key. Falls back to the raw endpoint when the
 * gateway sent something that is not host:port.
 */
export function ServiceConnectionLine({
  connection,
  endpoint,
  className,
  testId,
}: {
  connection: ServiceConnection | null;
  endpoint: string;
  className?: string;
  testId?: string;
}): ReactElement {
  const { status, copy } = useCopyFeedback();
  if (!connection) {
    return (
      <p className={cn("font-mono text-sm wrap-anywhere", className)} data-testid={testId}>
        {endpoint}
      </p>
    );
  }
  const label = copyButtonLabel(connection, status);
  return (
    <div
      data-slot="well"
      data-protocol={connection.protocol}
      data-testid={testId}
      className={cn(
        "flex min-w-0 flex-wrap items-center gap-x-3 gap-y-2 rounded-sm border border-border/70 bg-muted/20 px-3 py-2",
        className,
      )}
    >
      <Badge variant="outline" className="font-mono tracking-wide" title={`Connect over ${connection.label}`}>
        {connection.label}
      </Badge>
      {connection.href ? (
        <a
          href={connection.href}
          target="_blank"
          rel="noreferrer"
          className="inline-flex min-w-0 flex-[0_1_auto] items-center gap-1.5 font-mono text-sm underline-offset-4 wrap-anywhere hover:underline focus-visible:underline"
        >
          <span className="min-w-0">{connection.display}</span>
          <ExternalLink className="size-3.5 shrink-0" aria-hidden />
          <span className="sr-only">(opens in a new tab)</span>
        </a>
      ) : (
        <code className="min-w-0 flex-[0_1_auto] font-mono text-sm break-words">{connection.display}</code>
      )}
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="ms-auto h-auto min-h-8 whitespace-normal"
        onClick={() => void copy(connection.copyText)}
      >
        {status === "copied" ? <Check className="size-4" aria-hidden /> : <Copy className="size-4" aria-hidden />}
        {label}
      </Button>
      <span className="sr-only" aria-live="polite">
        {status === "copied" ? `${connection.protocol === "http" ? "URL" : "Command"} copied` : status === "failed" ? "Copy failed" : ""}
      </span>
    </div>
  );
}

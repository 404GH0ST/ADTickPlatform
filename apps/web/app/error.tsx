"use client";

import { useEffect } from "react";

import { Button } from "@/components/ui/button";
import { SystemState } from "@/components/ui/system-state";

export default function RouteError({ error, retry }: { error: unknown; retry: () => void }) {
  useEffect(() => {
    console.error(error);
  }, [error]);

  // Error boundaries can't export metadata. Focus moves here so keyboard and
  // screen reader users land on the explanation instead of a removed control.
  useEffect(() => {
    const previousTitle = document.title;
    document.title = "Something Went Wrong | AD Platform";
    document.querySelector<HTMLElement>("main h1")?.focus();
    return () => {
      document.title = previousTitle;
    };
  }, []);

  const digest =
    error instanceof Error && "digest" in error && typeof error.digest === "string"
      ? error.digest
      : "";

  return (
    <SystemState
      title="This page couldn't load"
      description={
        <>
          Something went wrong on our side. Try again, and if it keeps happening, tell the organizers
          {digest ? (
            <>
              {" "}and quote reference <code className="wrap-anywhere font-mono text-foreground">{digest}</code>
            </>
          ) : null}
          .
        </>
      }
      actions={
        <>
          <Button type="button" onClick={() => retry()}>
            Try again
          </Button>
          <Button asChild variant="outline">
            {/* A full load, because the app state behind this error may be broken. */}
            <a href="/">Go to Overview</a>
          </Button>
        </>
      }
    />
  );
}

"use client";

import type { ComponentProps } from "react";

import { Button } from "@/components/ui/button";

/** A full reload, so the session and every server check run again. */
export function RetryButton({ variant, size }: Pick<ComponentProps<typeof Button>, "variant" | "size">) {
  return (
    <Button type="button" variant={variant} size={size} onClick={() => window.location.reload()}>
      Try again
    </Button>
  );
}

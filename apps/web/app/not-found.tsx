import type { Metadata } from "next";
import Link from "next/link";

import { Button } from "@/components/ui/button";
import { SystemState } from "@/components/ui/system-state";

export const metadata: Metadata = { title: "Page Not Found" };

export default function NotFound() {
  return (
    <SystemState
      title="Page not found"
      description="That address doesn't match a page on this platform. Check the link, or head back to the overview."
      actions={
        <>
          <Button asChild>
            <Link href="/">Go to Overview</Link>
          </Button>
          <Button asChild variant="outline">
            <Link href="/docs/participant">Open Manual</Link>
          </Button>
        </>
      }
    />
  );
}

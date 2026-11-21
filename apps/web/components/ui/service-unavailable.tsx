import { Button } from "@/components/ui/button";
import { RetryButton } from "@/components/ui/retry-button";
import { SystemState } from "@/components/ui/system-state";

export function ServiceUnavailable() {
  return (
    <SystemState
      title="Platform unavailable"
      description="The platform API can't be reached, so your access couldn't be checked. You have not been signed out. Try again in a moment."
      actions={
        <>
          <RetryButton />
          <Button asChild variant="outline">
            <a href="/">Go to Overview</a>
          </Button>
        </>
      }
    />
  );
}

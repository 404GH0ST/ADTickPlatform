import type { Metadata } from "next";
import { redirect } from "next/navigation";

import { ServiceUnavailable } from "@/components/ui/service-unavailable";
import { getParticipantSession } from "@/lib/platform-api";

export const metadata: Metadata = {
  title: "Platform Unavailable",
  robots: { index: false },
};

export const dynamic = "force-dynamic";

/** Target of the proxy's rewrite when an organizer route can't be authorized. */
export default async function ServiceUnavailablePage() {
  const session = await getParticipantSession();
  if (session.reason !== "unavailable") {
    redirect("/");
  }
  return <ServiceUnavailable />;
}

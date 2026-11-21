import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { ServiceUnavailable } from "@/components/ui/service-unavailable";
import { getParticipantSession } from "@/lib/platform-api";

// Keeps an organizer tab apart from the participant page of the same name.
export const metadata: Metadata = {
  title: { default: "Organizer | AD Platform", template: "%s | Organizer | AD Platform" },
};

export default async function AdminLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const session = await getParticipantSession();

  // The role is unknown, so the organizer pages stay closed without a sign-in bounce.
  if (session.reason === "unavailable") {
    return <ServiceUnavailable />;
  }

  if (!session.authenticated) {
    redirect("/login");
  }

  if (session.role !== "organizer") {
    redirect("/");
  }

  return <>{children}</>;
}

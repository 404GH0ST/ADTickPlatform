import { redirect } from "next/navigation";
import { getParticipantSession } from "@/lib/platform-api";

export default async function AdminLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const session = await getParticipantSession();

  if (!session.authenticated) {
    redirect("/login");
  }

  if (session.role !== "organizer") {
    redirect("/");
  }

  return <>{children}</>;
}

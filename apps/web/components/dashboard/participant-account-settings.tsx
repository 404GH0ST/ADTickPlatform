"use client";

import type { FormEvent, ReactNode } from "react";
import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Settings } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { StatusBanner } from "@/components/ui/status-banner";
import { useUnsavedGuard } from "@/components/ui/unsaved-guard";
import type { PlatformOverview } from "@/lib/dashboard-types";
import {
  browserRequestErrorMessage,
  parseApiError,
} from "@/lib/api-utils";

type TeamMember = {
  player_id: number;
  display_name: string;
  email: string;
  role: string;
};
type Draft = {
  displayName: string;
  email: string;
  teamName: string;
  teamContactEmail: string;
};

type PasswordDraft = {
  currentPassword: string;
  newPassword: string;
  confirmPassword: string;
};

export function ParticipantAccountSettings({
  overview,
}: {
  overview: PlatformOverview;
}) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [pending, setPending] = useState(false);
  const unsaved = useUnsavedGuard({ open, onClose: () => setOpen(false), busy: pending });
  const trigger = useRef<HTMLButtonElement>(null);
  const closeButton = useRef<HTMLButtonElement>(null);
  const transferButton = useRef<HTMLButtonElement>(null);
  const keepCaptainButton = useRef<HTMLButtonElement>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [draft, setDraft] = useState<Draft>(() => draftFromOverview(overview));
  const [passwordDraft, setPasswordDraft] = useState<PasswordDraft>({
    currentPassword: "",
    newPassword: "",
    confirmPassword: "",
  });
  const [members, setMembers] = useState<TeamMember[]>([]);
  const [membersLoading, setMembersLoading] = useState(false);
  const [membersError, setMembersError] = useState<string | null>(null);
  const [membersAttempt, setMembersAttempt] = useState(0);
  const [confirmingTransfer, setConfirmingTransfer] = useState(false);
  const [transferPlayerID, setTransferPlayerID] = useState<number | "">("");
  const hasTeam = (overview.teamID ?? 0) > 0 && overview.role !== "organizer";
  const canEditTeam = hasTeam && overview.role === "captain";
  // Exclude the current captain (role "captain") so the list is correct even
  // when overview.playerID is undefined, plus organizers who can never captain.
  const transferableMembers = members.filter(
    (member) =>
      member.role !== "organizer" &&
      member.role !== "captain" &&
      member.player_id !== overview.playerID,
  );
  const transferTarget = transferableMembers.find(
    (member) => member.player_id === transferPlayerID,
  );

  // Reset the form only when the dialog transitions to open — not on every
  // overview refresh — so a success banner survives the router.refresh() that
  // follows a save or captain transfer.
  useEffect(() => {
    if (!open) {
      return;
    }
    setDraft(draftFromOverview(overview));
    setPasswordDraft({
      currentPassword: "",
      newPassword: "",
      confirmPassword: "",
    });
    setMessage(null);
    setError(null);
    setTransferPlayerID("");
    setConfirmingTransfer(false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  useEffect(() => {
    if (confirmingTransfer) keepCaptainButton.current?.focus();
  }, [confirmingTransfer]);

  // Load team members for captains while the dialog is open.
  useEffect(() => {
    if (!open || !canEditTeam) {
      setMembers([]);
      return;
    }

    let cancelled = false;
    setMembersLoading(true);
    setMembersError(null);
    void (async () => {
      try {
        const response = await fetch("/api/platform/session/team/members", {
          cache: "no-store",
        });
        if (!response.ok) {
          throw new Error(
            await parseApiError(
              response,
              "/api/platform/session/team/members",
            ),
          );
        }
        const payload = (await response.json()) as TeamMember[];
        if (!cancelled) {
          setMembers(Array.isArray(payload) ? payload : []);
        }
      } catch (loadError) {
        if (!cancelled) {
          setMembers([]);
          setMembersError(
            browserRequestErrorMessage(loadError, "failed to load team members"),
          );
        }
      } finally {
        if (!cancelled) {
          setMembersLoading(false);
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [open, canEditTeam, membersAttempt]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setMessage(null);
    setError(null);

    const wantsPasswordChange =
      passwordDraft.currentPassword !== "" ||
      passwordDraft.newPassword !== "" ||
      passwordDraft.confirmPassword !== "";
    const passwordIssue = wantsPasswordChange ? findPasswordIssue(passwordDraft) : null;
    if (passwordIssue) {
      setError(passwordIssue.message);
      document.getElementById(passwordIssue.fieldId)?.focus();
      return;
    }

    const saved = draftFromOverview(overview);
    const profileChanged = (Object.keys(saved) as (keyof Draft)[]).some(
      (key) => saved[key] !== draft[key],
    );
    let profileSaved = false;
    setPending(true);

    try {
      if (profileChanged || !wantsPasswordChange) {
        const response = await fetch("/api/platform/session/profile", {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            display_name: draft.displayName,
            email: draft.email,
            ...(canEditTeam
              ? {
                  team_name: draft.teamName,
                  team_contact_email: draft.teamContactEmail,
                }
              : {}),
          }),
        });
        if (!response.ok) {
          throw new Error(
            await parseApiError(response, "/api/platform/session/profile"),
          );
        }
        profileSaved = true;
      }

      if (wantsPasswordChange) {
        const passwordResponse = await fetch("/api/platform/session/password", {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            current_password: passwordDraft.currentPassword,
            new_password: passwordDraft.newPassword,
          }),
        });
        if (!passwordResponse.ok) {
          throw new Error(
            await parseApiError(
              passwordResponse,
              "/api/platform/session/password",
            ),
          );
        }
        setPasswordDraft({
          currentPassword: "",
          newPassword: "",
          confirmPassword: "",
        });
        setMessage(profileSaved ? "Account settings and password saved." : "Password changed.");
      } else {
        setMessage("Account settings saved.");
      }
      unsaved.markSaved();
      router.refresh();
    } catch (updateError) {
      if (profileSaved) {
        const reason = browserRequestErrorMessage(updateError, "password change failed");
        setMessage("Profile saved.");
        setError(`Password not changed: ${reason}`);
        router.refresh();
      } else {
        setError(
          browserRequestErrorMessage(updateError, "profile update failed"),
        );
      }
    } finally {
      setPending(false);
    }
  }

  async function transferCaptain() {
    if (transferPlayerID === "" || transferPlayerID <= 0) {
      setError("choose a teammate to receive captainship");
      return;
    }
    setConfirmingTransfer(false);
    setPending(true);
    setMessage(null);
    setError(null);
    let transferred = false;
    try {
      const response = await fetch("/api/platform/session/team/captain", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ player_id: transferPlayerID }),
      });
      if (!response.ok) {
        throw new Error(
          await parseApiError(response, "/api/platform/session/team/captain"),
        );
      }
      // Keep the dialog open so the success banner is visible; router.refresh()
      // re-renders this section as a former captain (a plain team member).
      setMessage("Captainship transferred. You are now a team member.");
      setTransferPlayerID("");
      transferred = true;
      router.refresh();
    } catch (transferError) {
      setError(
        browserRequestErrorMessage(transferError, "captain transfer failed"),
      );
    } finally {
      setPending(false);
      requestAnimationFrame(() =>
        (transferred ? closeButton : transferButton).current?.focus(),
      );
    }
  }

  return (
      <Dialog open={open} onOpenChange={(next) => (next ? setOpen(true) : unsaved.requestClose())}>
        <DialogTrigger asChild>
          <Button
            aria-label="Account settings"
            variant="outline"
            onClick={(event) => {
              trigger.current = event.currentTarget;
            }}
          >
            <Settings className="h-4 w-4" aria-hidden />
            Account
          </Button>
        </DialogTrigger>
        <DialogContent
          className="max-h-[90vh] overflow-y-auto"
          onChangeCapture={(event) => {
            // Picking a teammate isn't a profile edit worth protecting.
            if ((event.target as HTMLElement).id !== "participant-transfer-captain") {
              unsaved.markChanged();
            }
          }}
          onCloseAutoFocus={(event) => {
            const summary = trigger.current?.closest("details:not([open])")?.querySelector("summary");
            if (summary) {
              event.preventDefault();
              summary.focus();
            }
          }}
        >
          <DialogHeader>
            <DialogTitle>Account settings</DialogTitle>
            <DialogDescription>
              Update your player profile and account security.
            </DialogDescription>
          </DialogHeader>
          <form className="space-y-4" onSubmit={submit}>
            <section className="grid gap-3">
              <div>
                <p className="text-sm font-medium text-foreground">
                  Player information
                </p>
                <p className="text-sm text-muted-foreground">
                  Used for sign-in, audit entries, and WireGuard ownership.
                </p>
              </div>
              <Field label="Display name" htmlFor="participant-display-name">
                <Input
                  id="participant-display-name"
                  autoComplete="name"
                  value={draft.displayName}
                  onChange={(event) =>
                    setDraft({ ...draft, displayName: event.target.value })
                  }
                  required
                />
              </Field>
              <Field label="Email" htmlFor="participant-email">
                <Input
                  id="participant-email"
                  type="email"
                  autoComplete="email"
                  value={draft.email}
                  onChange={(event) =>
                    setDraft({ ...draft, email: event.target.value })
                  }
                  required
                />
              </Field>
            </section>

            {canEditTeam ? (
              <section className="grid gap-3 border-t pt-4">
                <div>
                  <p className="text-sm font-medium text-foreground">
                    Team information
                  </p>
                  <p className="text-sm text-muted-foreground">
                    The team name is reflected in scoreboard and attack views.
                  </p>
                </div>
                <Field label="Team name" htmlFor="participant-team-name">
                  <Input
                    id="participant-team-name"
                    value={draft.teamName}
                    onChange={(event) =>
                      setDraft({ ...draft, teamName: event.target.value })
                    }
                    required
                  />
                </Field>
                <Field label="Team email" htmlFor="participant-team-email">
                  <Input
                    id="participant-team-email"
                    type="email"
                    value={draft.teamContactEmail}
                    onChange={(event) =>
                      setDraft({
                        ...draft,
                        teamContactEmail: event.target.value,
                      })
                    }
                    required
                  />
                </Field>
                <div className="grid gap-2 border-t pt-4">
                  <div>
                    <p className="text-sm font-medium text-foreground">
                      Transfer captain
                    </p>
                    <p className="text-sm text-muted-foreground">
                      Give captainship to another teammate. You become a member.
                    </p>
                  </div>
                  {membersLoading ? (
                    <p className="text-sm text-muted-foreground">
                      Loading teammates...
                    </p>
                  ) : membersError ? (
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <p role="status" className="min-w-0 wrap-anywhere text-sm text-foreground">
                        Couldn&apos;t load teammates. {membersError}
                      </p>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={() => setMembersAttempt((attempt) => attempt + 1)}
                      >
                        Try again
                      </Button>
                    </div>
                  ) : transferableMembers.length === 0 ? (
                    <p className="text-sm text-muted-foreground">
                      No other teammates available yet.
                    </p>
                  ) : (
                    <>
                    <div className="grid gap-2 sm:grid-cols-[1fr_auto] sm:items-end">
                      <Field
                        label="Teammate"
                        htmlFor="participant-transfer-captain"
                      >
                        <select
                          id="participant-transfer-captain"
                          className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                          value={
                            transferPlayerID === ""
                              ? ""
                              : String(transferPlayerID)
                          }
                          onChange={(event) => {
                            const value = event.target.value;
                            setConfirmingTransfer(false);
                            setTransferPlayerID(
                              value === "" ? "" : Number(value),
                            );
                          }}
                        >
                          <option value="">Select teammate</option>
                          {transferableMembers.map((member) => (
                            <option
                              key={member.player_id}
                              value={member.player_id}
                            >
                              {member.display_name} ({member.email})
                            </option>
                          ))}
                        </select>
                      </Field>
                      <Button
                        ref={transferButton}
                        type="button"
                        variant="outline"
                        disabled={pending || transferPlayerID === "" || confirmingTransfer}
                        onClick={() => setConfirmingTransfer(true)}
                      >
                        Transfer
                      </Button>
                    </div>
                    {confirmingTransfer && transferTarget ? (
                      <div role="alert" className="grid gap-3 rounded-md border border-border p-3">
                        <p className="wrap-anywhere text-sm font-medium text-foreground">
                          Make {transferTarget.display_name} the captain? You&apos;ll become a team member, and only the new captain can transfer it back.
                        </p>
                        <div className="flex flex-wrap gap-2">
                          <Button
                            ref={keepCaptainButton}
                            type="button"
                            variant="outline"
                            onClick={() => {
                              setConfirmingTransfer(false);
                              requestAnimationFrame(() => transferButton.current?.focus());
                            }}
                          >
                            Stay captain
                          </Button>
                          <Button
                            type="button"
                            className="button-danger-solid"
                            onClick={() => void transferCaptain()}
                          >
                            Make captain
                          </Button>
                        </div>
                      </div>
                    ) : null}
                    </>
                  )}
                </div>
              </section>
            ) : hasTeam ? (
              <StatusBanner
                message="Only the team captain can edit the team name and contact email."
                variant="warning"
              />
            ) : (
              <StatusBanner
                message="Join a team before editing team information."
                variant="warning"
              />
            )}

            <section className="grid gap-3 border-t pt-4">
              <div>
                <p className="text-sm font-medium text-foreground">
                  Change password
                </p>
                <p className="text-sm text-muted-foreground">
                  Leave blank to keep your current password.
                </p>
              </div>
              <Field label="Current password" htmlFor="participant-current-password">
                <Input
                  id="participant-current-password"
                  type="password"
                  autoComplete="current-password"
                  value={passwordDraft.currentPassword}
                  onChange={(event) =>
                    setPasswordDraft({
                      ...passwordDraft,
                      currentPassword: event.target.value,
                    })
                  }
                />
              </Field>
              <Field label="New password" htmlFor="participant-new-password">
                <Input
                  id="participant-new-password"
                  type="password"
                  autoComplete="new-password"
                  value={passwordDraft.newPassword}
                  onChange={(event) =>
                    setPasswordDraft({
                      ...passwordDraft,
                      newPassword: event.target.value,
                    })
                  }
                />
              </Field>
              <Field
                label="Confirm new password"
                htmlFor="participant-confirm-password"
              >
                <Input
                  id="participant-confirm-password"
                  type="password"
                  autoComplete="new-password"
                  value={passwordDraft.confirmPassword}
                  onChange={(event) =>
                    setPasswordDraft({
                      ...passwordDraft,
                      confirmPassword: event.target.value,
                    })
                  }
                />
              </Field>
            </section>

            {error ? <StatusBanner message={error} variant="error" /> : null}
            {message ? (
              <StatusBanner message={message} variant="success" />
            ) : null}

            <DialogFooter>
              {unsaved.prompt ?? (
                <>
                  <Button ref={closeButton} type="button" variant="outline" onClick={unsaved.requestClose}>
                    Close
                  </Button>
                  <Button type="submit" disabled={pending}>
                    {pending ? "Saving..." : "Save"}
                  </Button>
                </>
              )}
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
  );
}

/** Checked before anything is sent, so a bad password can't leave a half-applied save. */
function findPasswordIssue(
  password: PasswordDraft,
): { message: string; fieldId: string } | null {
  if (!password.currentPassword) {
    return {
      message: "Enter your current password to change it.",
      fieldId: "participant-current-password",
    };
  }
  if (password.newPassword.length < 8) {
    return {
      message: "Your new password must be at least 8 characters.",
      fieldId: "participant-new-password",
    };
  }
  if (password.newPassword !== password.confirmPassword) {
    return {
      message: "The new passwords don't match.",
      fieldId: "participant-confirm-password",
    };
  }
  return null;
}

function draftFromOverview(overview: PlatformOverview): Draft {
  return {
    displayName: overview.displayName ?? "",
    email: overview.email ?? "",
    teamName: overview.teamName ?? "",
    teamContactEmail: overview.teamContactEmail ?? "",
  };
}

function Field({
  children,
  htmlFor,
  label,
}: {
  children: ReactNode;
  htmlFor: string;
  label: string;
}) {
  return (
    <label className="grid gap-1.5 text-sm font-medium" htmlFor={htmlFor}>
      {label}
      {children}
    </label>
  );
}

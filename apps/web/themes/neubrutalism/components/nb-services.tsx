"use client";

import type { ReactNode } from "react";
import { Download, KeyRound, LoaderCircle, Lock, RotateCcw, TriangleAlert } from "lucide-react";

import {
  Button,
  cn,
  describeService,
  formatSLAState,
  StatusBanner,
  type ServiceRow,
  type ServicesViewProps,
} from "@/components/theme/kit";

import { NbMatchClock } from "./nb-match-clock";

function statusFill(service: ServiceRow, locked: boolean) {
  if (locked) return "nb-fill-lilac";
  if (service.status === "stable") return "nb-fill-aqua";
  if (service.status === "warming") return "nb-fill-yellow";
  return "nb-fill-pink";
}

function Fact({ label, value, tone }: { label: string; value: ReactNode; tone?: "good" | "bad" }) {
  return (
    <div className="grid gap-0.5">
      <dt className="nb-eyebrow text-[0.6875rem] text-muted-foreground">{label}</dt>
      <dd className={cn("text-sm font-semibold", tone === "good" && "text-positive", tone === "bad" && "text-negative")}>
        {value}
      </dd>
    </div>
  );
}

/**
 * Service cards with a colored status header. Every button calls the stock
 * handler and takes its enabled/pending state from describeService(), so
 * locks, cooldowns and SSH unlocks behave exactly like the stock view.
 */
export function NbServices({
  flagSubmit,
  rows,
  pendingAction,
  overview,
  serviceFeedback,
  onSelectPrimaryAction,
  onRestart,
  onSelectReset,
}: ServicesViewProps) {
  return (
    // Mobile: flag form, then your services, then the clock. From sm up the
    // clock sits under the form, where a wide strip reads at a glance.
    <div className="flex flex-col gap-6">
      {flagSubmit}
      <div className="order-last sm:order-none">
        <NbMatchClock overview={overview} />
      </div>
      <section data-testid="nb-services" aria-label="Services" className="grid grid-cols-1 gap-4">
        <h2 className="wrap-anywhere text-2xl font-extrabold">Your services</h2>
        {rows.length === 0 ? (
          <p className="nb-panel p-6 text-sm text-muted-foreground">
            No services are assigned to your team yet. This page refreshes on its own once an organizer deploys a
            challenge.
          </p>
        ) : (
          <ul className="grid grid-cols-1 gap-6 xl:grid-cols-2">
            {rows.map((service) => {
              const view = describeService(service, {
                feedback: serviceFeedback(service),
                pendingAction,
                matchState: overview.matchState,
              });
              const statusLabel = view.lock ? view.lock.label : service.status;
              return (
                <li
                  key={service.id}
                  data-testid={`nb-service-${service.id}`}
                  className="nb-panel flex flex-col overflow-hidden xl:only:col-span-2"
                >
                  <div
                    className={cn(
                      "flex flex-wrap items-start justify-between gap-3 border-b-[3px] border-border px-5 py-4",
                      statusFill(service, Boolean(view.lock)),
                    )}
                  >
                    <div className="grid min-w-0 gap-0.5">
                      <h3 className="wrap-anywhere text-2xl font-extrabold leading-tight">{service.name}</h3>
                      <p className="wrap-anywhere font-mono text-sm font-medium">{service.endpoint}</p>
                    </div>
                    <span className={cn("nb-sticker nb-status-chip", statusFill(service, Boolean(view.lock)))}>
                      {view.lock ? <Lock className="size-3.5" strokeWidth={2.6} aria-hidden /> : null}
                      {statusLabel}
                    </span>
                  </div>

                  <div className="grid flex-1 gap-4 px-5 py-4">
                    <dl className="grid grid-cols-2 gap-x-6 gap-y-3 sm:grid-cols-[repeat(auto-fit,minmax(9rem,1fr))]">
                      <Fact
                        label="Checker"
                        value={service.checker}
                        tone={service.checker === "passing" ? "good" : "bad"}
                      />
                      <Fact
                        label="SLA"
                        value={formatSLAState(service)}
                        tone={service.slaStatus === "ok" ? "good" : service.slaStatus === "unknown" ? undefined : "bad"}
                      />
                      <Fact label="SSH" value={service.unlocked ? "unlocked" : "locked"} />
                      <Fact label="Reset" value={view.cooldownLabel} />
                      <Fact label="Last event" value={service.lastEvent} />
                    </dl>
                    {service.slaMessage ? (
                      <p className="text-xs leading-5 text-muted-foreground">{service.slaMessage}</p>
                    ) : null}

                    {view.notice ? (
                      <div data-testid={`service-action-notice-${service.id}`}>
                        <StatusBanner message={view.notice.message} variant={view.notice.variant} />
                      </div>
                    ) : null}

                    {view.lock || view.unlockHint ? (
                      <p className="flex gap-2 text-sm text-muted-foreground">
                        <TriangleAlert className="mt-0.5 size-4 shrink-0" strokeWidth={2.4} aria-hidden />
                        <span>{view.lock ? view.lock.help : view.unlockHint}</span>
                      </p>
                    ) : null}
                  </div>

                  <div className="flex flex-wrap gap-2.5 border-t-2 border-dashed border-border px-5 py-4">
                    <Button
                      disabled={view.ssh.disabled}
                      className={cn(!service.unlocked && view.ssh.disabled && "nb-key-locked")}
                      onClick={() => onSelectPrimaryAction(service)}
                    >
                      {view.ssh.pending ? (
                        <LoaderCircle className="size-4 animate-spin" aria-hidden />
                      ) : !service.unlocked && view.ssh.disabled ? (
                        <Lock className="size-4" strokeWidth={2.4} aria-hidden />
                      ) : (
                        <KeyRound className="size-4" strokeWidth={2.4} aria-hidden />
                      )}
                      {view.ssh.label}
                    </Button>
                    <Button variant="secondary" disabled={view.restart.disabled} onClick={() => onRestart(service)}>
                      {view.restart.pending ? (
                        <LoaderCircle className="size-4 animate-spin" aria-hidden />
                      ) : (
                        <RotateCcw className="size-4" strokeWidth={2.4} aria-hidden />
                      )}
                      Restart{view.restart.cooldownSeconds > 0 ? ` ${view.restart.cooldownSeconds}s` : ""}
                    </Button>
                    <Button
                      variant="outline"
                      className="button-danger-subtle"
                      disabled={view.reset.disabled}
                      onClick={() => onSelectReset(service)}
                    >
                      {view.reset.pending ? <LoaderCircle className="size-4 animate-spin" aria-hidden /> : null}
                      Factory Reset{view.reset.cooldownSeconds > 0 ? ` ${view.reset.cooldownSeconds}s` : ""}
                    </Button>
                    {view.sourceHref ? (
                      <Button variant="outline" asChild>
                        <a href={view.sourceHref} download>
                          <Download className="size-4" strokeWidth={2.4} aria-hidden />
                          Download Source
                        </a>
                      </Button>
                    ) : null}
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </section>
    </div>
  );
}

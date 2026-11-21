"use client";

import type { ReactNode } from "react";
import { Check, Copy, Download, ExternalLink, KeyRound, LoaderCircle, Lock, RotateCcw, TriangleAlert } from "lucide-react";

import {
  Button,
  ChallengeCategoryIcon,
  cn,
  copyButtonLabel,
  describeService,
  formatSLAState,
  StatusBanner,
  useCopyFeedback,
  type ServiceConnection,
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
    <div className="grid min-w-0 gap-0.5 wrap-anywhere">
      <dt className="nb-eyebrow text-[0.6875rem] text-muted-foreground">{label}</dt>
      <dd className={cn("text-sm font-semibold", tone === "good" && "text-positive", tone === "bad" && "text-negative")}>
        {value}
      </dd>
    </div>
  );
}

/**
 * How to connect, as a zone at the top of the card: a neutral protocol sticker
 * (protocol is not a status, so it takes no status hue), then the URL (HTTP,
 * openable) or nc command (TCP), and a copy key.
 */
function NbConnection({ connection, endpoint, serviceId }: { connection: ServiceConnection | null; endpoint: string; serviceId: string }) {
  const { status, copy } = useCopyFeedback();
  if (!connection) {
    return (
      <p data-testid={`service-connection-${serviceId}`} className="font-mono text-sm font-medium wrap-anywhere">
        {endpoint}
      </p>
    );
  }
  return (
    <div
      data-slot="well"
      data-protocol={connection.protocol}
      data-testid={`service-connection-${serviceId}`}
      className="nb-connection flex min-w-0 flex-wrap items-center gap-x-3 gap-y-2 px-3 py-2.5"
    >
      <span className="nb-sticker nb-stat font-mono tracking-wider" title={`Connect over ${connection.label}`}>
        {connection.label}
      </span>
      {connection.href ? (
        <a
          href={connection.href}
          target="_blank"
          rel="noreferrer"
          className="nb-connection-target inline-flex min-w-0 flex-[0_1_auto] items-center gap-1.5 font-mono text-sm font-semibold wrap-anywhere"
        >
          <span className="min-w-0">{connection.display}</span>
          <ExternalLink className="size-3.5 shrink-0" strokeWidth={2.4} aria-hidden />
          <span className="sr-only">(opens in a new tab)</span>
        </a>
      ) : (
        <span className="nb-connection-target min-w-0 flex-[0_1_auto] font-mono text-sm font-semibold break-words">
          {connection.display}
        </span>
      )}
      <Button type="button" variant="outline" size="sm" className="ms-auto" onClick={() => void copy(connection.copyText)}>
        {status === "copied" ? (
          <Check className="size-4" strokeWidth={2.4} aria-hidden />
        ) : (
          <Copy className="size-4" strokeWidth={2.4} aria-hidden />
        )}
        {copyButtonLabel(connection, status)}
      </Button>
      <span className="sr-only" aria-live="polite">
        {status === "copied"
          ? `${connection.protocol === "http" ? "URL" : "Command"} copied`
          : status === "failed"
            ? "Copy failed"
            : ""}
      </span>
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
            {overview.servicesUnavailable
              ? "Your services can't be loaded right now. This doesn't mean they were removed. They'll return once the platform API responds."
              : "No services are assigned to your team yet. This page refreshes on its own once an organizer deploys a challenge."}
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
                  className="nb-panel @container flex flex-col overflow-hidden xl:only:col-span-2"
                >
                  <div
                    className={cn(
                      "flex flex-wrap items-start justify-between gap-3 border-b-[3px] border-border px-5 py-4",
                      statusFill(service, Boolean(view.lock)),
                    )}
                  >
                    <div className="flex min-w-0 flex-[1_1_12rem] items-center gap-3 @max-[18rem]:flex-col @max-[18rem]:items-start">
                      <span className="nb-icon-key grid size-11 shrink-0 place-items-center rounded-[var(--nb-radius-sm)]">
                        <ChallengeCategoryIcon category={service.category} className="size-6" strokeWidth={2.2} />
                      </span>
                      <div className="grid min-w-0 gap-0.5">
                        <p className="nb-category" data-testid={`service-category-${service.id}`}>
                          {view.categoryLabel}
                        </p>
                        <h3 className="wrap-anywhere text-2xl font-extrabold leading-tight">{service.name}</h3>
                      </div>
                    </div>
                    <span className={cn("nb-sticker nb-status-chip", statusFill(service, Boolean(view.lock)))}>
                      {view.lock ? <Lock className="size-3.5" strokeWidth={2.6} aria-hidden /> : null}
                      {statusLabel}
                    </span>
                  </div>

                  <div className="grid min-w-0 flex-1 gap-4 px-5 py-4 @max-[12rem]:px-2">
                    <NbConnection connection={view.connection} endpoint={service.endpoint} serviceId={service.id} />
                    <dl className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,7rem),1fr))] gap-x-6 gap-y-3 sm:grid-cols-[repeat(auto-fit,minmax(min(100%,9rem),1fr))]">
                      <Fact
                        label="Checker"
                        value={service.checker === "passing" ? "passing" : "warning (failing)"}
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
                      <p className="text-xs leading-5 wrap-anywhere text-muted-foreground">{service.slaMessage}</p>
                    ) : null}

                    {view.notice ? (
                      <div data-testid={`service-action-notice-${service.id}`}>
                        <StatusBanner message={view.notice.message} variant={view.notice.variant} />
                      </div>
                    ) : null}

                    {view.lock || view.unlockHint ? (
                      <p className="flex gap-2 text-sm text-muted-foreground @max-[10rem]:block">
                        <TriangleAlert
                          className="mt-0.5 size-4 shrink-0 @max-[10rem]:me-2 @max-[10rem]:inline @max-[10rem]:align-text-bottom"
                          strokeWidth={2.4}
                          aria-hidden
                        />
                        <span className="min-w-0 wrap-anywhere">{view.lock ? view.lock.help : view.unlockHint}</span>
                      </p>
                    ) : null}
                  </div>

                  <div className="flex flex-wrap gap-2.5 border-t-2 border-dashed border-border px-5 py-4 @max-[12rem]:px-2 @max-[12rem]:[&>[data-slot=button]]:px-2">
                    <Button
                      disabled={view.ssh.disabled}
                      holdFocus={view.ssh.pending}
                      className={cn(!service.unlocked && view.ssh.disabled && "nb-key-locked")}
                      onClick={() => onSelectPrimaryAction(service)}
                    >
                      {view.ssh.pending ? (
                        <LoaderCircle className="size-4 motion-safe:animate-spin" aria-hidden />
                      ) : !service.unlocked && view.ssh.disabled ? (
                        <Lock className="size-4" strokeWidth={2.4} aria-hidden />
                      ) : (
                        <KeyRound className="size-4" strokeWidth={2.4} aria-hidden />
                      )}
                      {view.ssh.label}
                    </Button>
                    <Button variant="secondary" disabled={view.restart.disabled} holdFocus={view.restart.pending} onClick={() => onRestart(service)}>
                      {view.restart.pending ? (
                        <LoaderCircle className="size-4 motion-safe:animate-spin" aria-hidden />
                      ) : (
                        <RotateCcw className="size-4" strokeWidth={2.4} aria-hidden />
                      )}
                      Restart{view.restart.cooldownSeconds > 0 ? ` ${view.restart.cooldownSeconds}s` : ""}
                    </Button>
                    <Button
                      variant="outline"
                      className="button-danger-subtle"
                      disabled={view.reset.disabled}
                      holdFocus={view.reset.pending}
                      onClick={() => onSelectReset(service)}
                    >
                      {view.reset.pending ? <LoaderCircle className="size-4 motion-safe:animate-spin" aria-hidden /> : null}
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

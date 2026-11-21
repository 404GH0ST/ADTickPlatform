"use client";

import Link from "next/link";
import { useEffect, useRef, useSyncExternalStore } from "react";
import {
  BookOpen,
  Boxes,
  Flag,
  Gamepad2,
  LayoutGrid,
  type LucideIcon,
  PanelLeftClose,
  PanelLeftOpen,
  Rocket,
  ScrollText,
  Server,
  Trophy,
  UserRound,
  UsersRound,
  Zap,
} from "lucide-react";

import { AppearanceControls, LinkPendingSignal, type AppShellProps, type NavItem } from "@/components/theme/kit";

import { NbMenu } from "./nb-menu";

const ICONS: Record<string, LucideIcon> = {
  "/": LayoutGrid,
  "/scoreboard": Trophy,
  "/services": Server,
  "/attacks": Zap,
  "/docs/participant": BookOpen,
  "/admin": LayoutGrid,
  "/admin/game": Gamepad2,
  "/admin/scoreboard": Trophy,
  "/admin/attacks": Zap,
  "/admin/submissions": Flag,
  "/admin/teams": UsersRound,
  "/admin/players": UserRound,
  "/admin/challenges": Boxes,
  "/admin/deployments": Rocket,
  "/admin/audit": ScrollText,
};

const RAIL_STORAGE_KEY = "ad-platform-rail";
const RAIL_EVENT = "ad-platform-rail-change";

// The collapsed flag lives on <html data-rail> (set before paint by the root
// layout's init script), so CSS lays the rail out correctly on every page
// load and client navigation without waiting for React.
function subscribeRail(onChange: () => void) {
  window.addEventListener(RAIL_EVENT, onChange);
  return () => window.removeEventListener(RAIL_EVENT, onChange);
}

function readRailCollapsed() {
  return document.documentElement.dataset.rail === "collapsed";
}

function useRailCollapsed() {
  const collapsed = useSyncExternalStore(subscribeRail, readRailCollapsed, () => false);
  const toggle = () => {
    const root = document.documentElement;
    const next = !readRailCollapsed();
    if (next) root.dataset.rail = "collapsed";
    else delete root.dataset.rail;
    try {
      window.localStorage.setItem(RAIL_STORAGE_KEY, next ? "collapsed" : "expanded");
    } catch {
      // Storage blocked: the toggle still works until the next full load.
    }
    window.dispatchEvent(new Event(RAIL_EVENT));
  };
  return { collapsed, toggle };
}

function NavGroup({
  label,
  items,
  activePath,
  collapsed,
}: {
  label?: string;
  items: readonly NavItem[];
  activePath: string;
  collapsed: boolean;
}) {
  return (
    <div className="contents lg:grid lg:gap-1">
      {label ? (
        <p className="nb-rail-group nb-eyebrow hidden px-2 pb-1 pt-2 text-[0.6875rem] lg:block">
          <span className="nb-rail-text">{label}</span>
        </p>
      ) : null}
      {items.map((item) => {
        const Icon = ICONS[item.href] ?? LayoutGrid;
        return (
          <Link
            key={item.href}
            href={item.href}
            data-active={activePath === item.href ? "true" : "false"}
            aria-current={activePath === item.href ? "page" : undefined}
            className="nb-nav-link"
            title={collapsed ? item.label : undefined}
          >
            <Icon className="size-[1.05rem] shrink-0" strokeWidth={2.25} aria-hidden />
            <span className="nb-rail-text min-w-0 wrap-anywhere">{item.label}</span>
            <LinkPendingSignal />
          </Link>
        );
      })}
    </div>
  );
}

function Brand({ organizer }: { organizer: boolean }) {
  return (
    <span className="flex items-center gap-3">
      <span className="nb-logo shrink-0" aria-hidden>
        AD
      </span>
      <span className="nb-rail-text grid min-w-0 wrap-anywhere leading-tight">
        <span className="text-lg font-bold">AD Platform</span>
        <span className="text-xs font-semibold">{organizer ? "Organizer desk" : "Team console"}</span>
      </span>
    </span>
  );
}

/**
 * One layout for both surfaces: a collapsible blue rail, then the yellow
 * title banner with the header controls beside it. Same props as the stock shell, so
 * every page keeps its behavior; below lg the rail becomes a scrolling strip.
 */
export function NbShell({
  surface = "participant",
  activePath,
  navItems,
  secondaryNavItems = [],
  title,
  children,
  headerActions,
  primaryHeaderAction,
  navActions,
  belowNav,
  summarySection,
  alertSection,
}: AppShellProps) {
  const organizer = surface === "organizer";
  const rail = useRailCollapsed();
  const navRef = useRef<HTMLElement>(null);
  useEffect(() => {
    const nav = navRef.current;
    if (!nav || nav.scrollWidth <= nav.clientWidth || nav.matches(":focus-within")) return;
    nav.querySelector<HTMLElement>('[aria-current="page"]')
      ?.scrollIntoView({ block: "nearest", inline: "nearest" });
  }, [activePath]);
  return (
    <div data-testid="nb-shell" className="nb-frame min-h-dvh text-foreground lg:grid">
      <a href="#nb-main" className="nb-skip">
        Skip to content
      </a>
      <div className="nb-rail-column p-3 lg:sticky lg:top-0 lg:h-dvh lg:p-0">
        <div className="nb-rail-motion">
          <aside
            id="nb-rail"
            className="nb-rail lg:flex lg:flex-col lg:overflow-y-auto lg:overflow-x-hidden"
          >
            <div className="nb-rail-brand flex items-center px-4 pb-1 pt-3 lg:px-5 lg:pb-5 lg:pt-5">
              <Link href={organizer ? "/admin" : "/"} className="min-w-0">
                <Brand organizer={organizer} />
              </Link>
            </div>
            <nav
              ref={navRef}
              aria-label="Primary"
              className="flex gap-1.5 overflow-x-auto scroll-px-3 px-3 pb-2.5 pt-1.5 lg:grid lg:gap-1 lg:overflow-visible lg:px-3 lg:pb-6 lg:pt-1"
              onFocus={(event) => {
                const nav = event.currentTarget;
                const link = event.target.closest<HTMLAnchorElement>("a.nb-nav-link");
                if (nav.scrollWidth <= nav.clientWidth || !link?.matches(":focus-visible")) return;
                requestAnimationFrame(() => {
                  if (document.activeElement === link) {
                    link.scrollIntoView({ block: "nearest", inline: "nearest", behavior: "instant" });
                  }
                });
              }}
            >
              <NavGroup
                label={organizer ? "Match" : undefined}
                items={navItems}
                activePath={activePath}
                collapsed={rail.collapsed}
              />
              {secondaryNavItems.length > 0 ? (
                <>
                  <span aria-hidden className="mx-1 my-2 w-[3px] shrink-0 self-stretch bg-[var(--nb-on-blue)] opacity-60 lg:hidden" />
                  <NavGroup label="Setup" items={secondaryNavItems} activePath={activePath} collapsed={rail.collapsed} />
                </>
              ) : null}
            </nav>
            <div className="mt-auto hidden px-3 pb-4 lg:block">
              <button
                type="button"
                className="nb-nav-link nb-rail-toggle w-full"
                aria-controls="nb-rail"
                aria-expanded={!rail.collapsed}
                aria-label={rail.collapsed ? "Expand sidebar" : "Collapse sidebar"}
                title={rail.collapsed ? "Expand sidebar" : "Collapse sidebar"}
                onClick={rail.toggle}
              >
                {rail.collapsed ? (
                  <PanelLeftOpen className="size-[1.05rem] shrink-0" strokeWidth={2.25} aria-hidden />
                ) : (
                  <PanelLeftClose className="size-[1.05rem] shrink-0" strokeWidth={2.25} aria-hidden />
                )}
                <span className="nb-rail-text">Collapse</span>
              </button>
            </div>
          </aside>
        </div>
      </div>

      <div className="flex min-w-0 flex-col gap-6 px-3 pb-10 pt-1 sm:px-6 lg:px-8 lg:pt-4">
        {/* Title banner and header keys share one row when there is room, so
            content starts one banner-height from the top, not two. On phones
            the keys fold to icons, so two icon keys still fit beside the
            banner; a text key such as Sign In needs more room than the title
            can give up, so the keys then wrap to their own row under it. */}
        <header className="relative flex flex-wrap items-center gap-x-3 gap-y-4 sm:gap-x-5">
          <div className="nb-banner flex min-w-[min(100%,12rem)] flex-1 flex-wrap items-center justify-between gap-3 px-4 py-3 sm:min-w-[min(100%,20rem)] sm:px-6 sm:py-4">
            <h1 className="min-w-0 max-w-full wrap-anywhere text-balance text-2xl font-extrabold leading-none tracking-[-0.03em] sm:text-4xl">{title}</h1>
            {navActions ? <div className="flex min-w-0 max-w-full flex-wrap items-center gap-2.5">{navActions}</div> : null}
          </div>
          <div className="nb-header-actions relative flex max-w-full flex-wrap items-center gap-2.5">
            <AppearanceControls />
            {/* Organizers keep their keys in view from sm up; on phones they
                fold into the same Menu key participants use, so the header
                stays one row beside the banner. */}
            {organizer && headerActions ? (
              <>
                <div className="contents max-sm:hidden">{headerActions}</div>
                <div className="sm:hidden">
                  <NbMenu>{headerActions}</NbMenu>
                </div>
              </>
            ) : headerActions ? (
              <NbMenu>{headerActions}</NbMenu>
            ) : null}
            {primaryHeaderAction}
          </div>
        </header>
        <main id="nb-main" tabIndex={-1} className="flex min-h-0 flex-1 flex-col gap-6 outline-none">
          {belowNav}
          {organizer || activePath === "/" ? summarySection : null}
          {alertSection}
          <div className="nb-content flex min-h-0 flex-1 flex-col gap-6">{children}</div>
        </main>
      </div>
    </div>
  );
}

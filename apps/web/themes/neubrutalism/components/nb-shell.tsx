"use client";

import Link from "next/link";
import { useSyncExternalStore } from "react";
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

import { AppearanceControls, type AppShellProps, type NavItem } from "@/components/theme/kit";

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
    // Animate only user toggles, never page loads or navigation.
    root.dataset.railAnimating = "true";
    window.setTimeout(() => delete root.dataset.railAnimating, 260);
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
            <span className="nb-rail-text">{item.label}</span>
          </Link>
        );
      })}
    </div>
  );
}

function Brand({ organizer }: { organizer: boolean }) {
  return (
    <span className="flex items-center gap-3">
      <span className="nb-logo" aria-hidden>
        AD
      </span>
      <span className="nb-rail-text grid leading-tight">
        <span className="text-lg font-bold">AD Platform</span>
        <span className="text-xs font-medium opacity-85">{organizer ? "Organizer desk" : "Team console"}</span>
      </span>
    </span>
  );
}

/**
 * One layout for both surfaces: a collapsible blue rail, a row of header
 * controls, then the yellow title banner. Same props as the stock shell, so
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
  navActions,
  belowNav,
  summarySection,
  alertSection,
}: AppShellProps) {
  const organizer = surface === "organizer";
  const rail = useRailCollapsed();
  return (
    <div data-testid="nb-shell" className="nb-frame min-h-dvh text-foreground lg:grid">
      <div className="p-3 lg:sticky lg:top-0 lg:h-dvh lg:p-4 lg:pr-0">
        <aside
          id="nb-rail"
          className="nb-rail lg:flex lg:h-full lg:flex-col lg:overflow-y-auto lg:overflow-x-hidden"
        >
          <div className="nb-rail-brand flex items-center px-4 pb-2 pt-4 lg:px-5 lg:pb-5 lg:pt-5">
            <Link href={organizer ? "/admin" : "/"} aria-label={organizer ? "Organizer overview" : "Overview"}>
              <Brand organizer={organizer} />
            </Link>
          </div>
          <nav
            aria-label="Primary"
            className="flex gap-1.5 overflow-x-auto px-3 pb-3 pt-1 lg:grid lg:gap-1 lg:overflow-visible lg:px-3 lg:pb-6"
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

      <div className="flex min-w-0 flex-col gap-6 px-3 pb-10 pt-1 sm:px-6 lg:px-8 lg:pt-4">
        <div className="flex flex-wrap items-center gap-2.5 lg:justify-end">
          <AppearanceControls />
          {headerActions}
        </div>
        <header className="nb-banner flex flex-wrap items-center justify-between gap-4 px-5 py-5 sm:px-7 sm:py-6">
          <h1 className="text-balance text-4xl font-extrabold leading-none tracking-[-0.03em] sm:text-5xl">{title}</h1>
          {navActions ? <div className="flex flex-wrap items-center gap-2.5">{navActions}</div> : null}
        </header>
        {belowNav}
        {organizer || activePath === "/" ? summarySection : null}
        {alertSection}
        <div className="nb-content flex min-h-0 flex-1 flex-col gap-6">{children}</div>
      </div>
    </div>
  );
}

'use client';

import { useId, type ReactElement } from 'react';
import { X } from 'lucide-react';

import {
  FIRST_BLOOD_BANNER_EXIT_MS,
  FIRST_BLOOD_BANNER_MAX_ROWS,
} from '@/lib/first-blood';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import type { CyberAttackEvent } from './cyber-attack-map';

export function FirstBloodBanner({
  attacks,
  leaving,
  reducedMotion,
  retained,
  onDismiss,
}: {
  /** The lead capture first; simultaneous first bloods follow as compact rows. */
  attacks: CyberAttackEvent[];
  leaving: boolean;
  reducedMotion: boolean;
  retained: boolean;
  onDismiss: () => void;
}): ReactElement | null {
  const headingId = useId();
  const [attack, ...others] = attacks;
  if (!attack) return null;
  const extras = others.slice(0, FIRST_BLOOD_BANNER_MAX_ROWS - 1);
  const hidden = others.length - extras.length;
  const institution = attack.attacker_institution?.trim() ?? '';
  const dense =
    others.length > 0 ||
    attack.attacker.length + attack.service.length + institution.length / 2 > 100;
  return (
    <div
      role="status"
      aria-atomic="true"
      aria-labelledby={headingId}
      data-testid={retained ? 'first-blood-result' : 'first-blood-banner'}
      data-first-blood-banner
      data-attack-id={attack.id}
      data-banner-phase={leaving ? 'exit' : 'hold'}
      data-reduced-motion={reducedMotion ? 'true' : 'false'}
      className="pointer-events-none absolute inset-x-3 top-1/2 z-20 -translate-y-1/2 sm:inset-x-6"
      style={{ color: 'var(--attack-map-node-label)' }}
    >
      <div
        data-first-blood-banner-panel="true"
        className={cn(
          'relative mx-auto grid max-w-[46rem] min-w-0 overflow-hidden border-y px-4 text-center sm:grid-cols-[minmax(0,0.55fr)_minmax(0,1fr)] sm:items-center sm:gap-8 sm:px-8 sm:py-7 sm:text-left',
          dense ? 'gap-3 py-4' : 'gap-4 py-5',
        )}
        style={{
          backgroundColor: 'var(--attack-map-background)',
          borderColor: 'var(--attack-map-legend-border)',
        }}
      >
        {reducedMotion ? null : (
          <>
            <span
              aria-hidden="true"
              data-first-blood-banner-slash="true"
              className="absolute inset-x-0 top-1/2 h-[3px]"
              style={{ backgroundColor: 'var(--signal-negative)' }}
            />
            <span
              aria-hidden="true"
              data-first-blood-banner-shine="true"
              className="absolute inset-y-0 left-0 w-1/3"
            />
          </>
        )}
        <div data-first-blood-banner-header className="contents">
          <h3
            id={headingId}
            className={cn(
              'font-semibold uppercase leading-tight tracking-[0.08em] sm:text-3xl',
              dense ? 'text-xl' : 'text-2xl',
            )}
            style={{ color: 'var(--signal-negative)' }}
          >
            {attacks.length > 1 ? `${attacks.length} first bloods` : 'First blood'}
          </h3>
          <Button
            type="button"
            variant="outline"
            size="icon"
            aria-label="Dismiss first blood"
            data-first-blood-dismiss
            className="hidden size-[44px]"
            onClick={onDismiss}
          >
            <X className="size-4" aria-hidden />
          </Button>
        </div>
        <div
          data-first-blood-banner-body="true"
          className={cn('min-w-0', dense ? 'space-y-2' : 'space-y-3')}
        >
          <div data-first-blood-banner-identity className="min-w-0">
            <p
              data-testid="first-blood-banner-team"
              className={cn(
                'text-balance font-semibold leading-tight [overflow-wrap:anywhere]',
                dense ? 'text-lg sm:text-3xl' : 'text-2xl sm:text-4xl',
              )}
            >
              {attack.attacker}
            </p>
            {institution ? (
              <p
                data-testid="first-blood-banner-institution"
                className={cn(
                  'mt-1 text-balance font-medium uppercase leading-snug tracking-[0.12em] [overflow-wrap:anywhere]',
                  dense ? 'text-[0.6875rem] sm:text-xs' : 'text-xs sm:text-sm',
                )}
                style={{ color: 'var(--attack-map-legend-foreground)' }}
              >
                {institution}
              </p>
            ) : null}
          </div>
          <p
            className={cn(
              'text-sm [overflow-wrap:anywhere] sm:text-base',
              dense ? 'leading-snug' : 'leading-relaxed',
            )}
          >
            First to capture{' '}
            <strong
              data-testid="first-blood-banner-challenge"
              className="font-semibold"
            >
              {attack.service}
            </strong>
          </p>
          <p
            className={cn(
              'text-xs leading-relaxed [overflow-wrap:anywhere]',
              dense ? 'hidden sm:block' : '',
            )}
          >
            Captured from {attack.victim}
            {attack.victim_institution ? (
              <span data-testid="first-blood-banner-victim-institution">
                {' '}({attack.victim_institution})
              </span>
            ) : null}
            <span aria-hidden="true"> · </span>Tick #{attack.tick}
          </p>
          {extras.length > 0 ? (
            <ul
              className="space-y-1 border-t pt-2 text-xs leading-snug sm:text-sm"
              style={{ borderColor: 'var(--attack-map-legend-border)' }}
            >
              {extras.map((entry) => (
                <li
                  key={entry.id}
                  data-testid="first-blood-banner-extra"
                  className="[overflow-wrap:anywhere]"
                >
                  <span className="font-semibold">{entry.attacker}</span>
                  {entry.attacker_institution ? (
                    <span
                      data-first-blood-banner-extra-institution
                      style={{ color: 'var(--attack-map-legend-foreground)' }}
                    >
                      {' '}· {entry.attacker_institution}
                    </span>
                  ) : null}
                  {' '}first on{' '}
                  <strong className="font-semibold">{entry.service}</strong>
                </li>
              ))}
              {hidden > 0 ? <li>and {hidden} more</li> : null}
            </ul>
          ) : null}
        </div>
      </div>
      <style>{`
        @keyframes ad-first-blood-banner-exit {
          0% { opacity: 1; transform: scaleY(1); }
          60% { opacity: 1; transform: scaleY(0.06) scaleX(1.03); }
          100% { opacity: 0; transform: scaleY(0.02) scaleX(0.4); }
        }
        @keyframes ad-first-blood-banner-rise {
          from { opacity: 0; transform: translateX(28px); filter: blur(4px); }
          to { opacity: 1; transform: translateX(0); filter: none; }
        }
        @keyframes ad-first-blood-banner-shine {
          from { transform: translateX(-120%) skewX(-20deg); }
          to { transform: translateX(420%) skewX(-20deg); }
        }
        @keyframes ad-first-blood-banner-glow {
          0% { box-shadow: 0 0 0 1px var(--signal-negative), 0 0 64px color-mix(in srgb, var(--signal-negative) 55%, transparent); }
          100% { box-shadow: 0 0 0 1px color-mix(in srgb, var(--signal-negative) 40%, transparent), 0 0 18px color-mix(in srgb, var(--signal-negative) 14%, transparent); }
        }
        @keyframes ad-first-blood-banner-breathe {
          0%, 100% { box-shadow: 0 0 0 1px color-mix(in srgb, var(--signal-negative) 40%, transparent), 0 0 18px color-mix(in srgb, var(--signal-negative) 14%, transparent); }
          50% { box-shadow: 0 0 0 1px color-mix(in srgb, var(--signal-negative) 65%, transparent), 0 0 30px color-mix(in srgb, var(--signal-negative) 26%, transparent); }
        }
        @keyframes ad-first-blood-banner-slam {
          0% { opacity: 0; transform: scale(1.22); filter: blur(6px); }
          45% { opacity: 1; transform: scale(0.97); filter: blur(0); }
          70% { transform: scale(1.01); }
          100% { opacity: 1; transform: scale(1); filter: none; }
        }
        @keyframes ad-first-blood-banner-slash {
          0% { transform: scaleX(0); opacity: 1; }
          45% { transform: scaleX(1); opacity: 1; }
          100% { transform: scaleX(1) scaleY(0); opacity: 0; }
        }
        @keyframes ad-first-blood-banner-heading {
          0% { letter-spacing: 0.6em; opacity: 0;
            text-shadow: 4px 0 var(--signal-negative), -4px 0 var(--attack-map-node-label); }
          35% { opacity: 1; text-shadow: -3px 0 var(--signal-negative), 3px 0 var(--attack-map-node-label); }
          55% { text-shadow: 2px 0 var(--signal-negative), -1px 0 var(--attack-map-node-label); }
          100% { letter-spacing: 0.08em; opacity: 1; text-shadow: none; }
        }
        [data-testid="first-blood-banner"][data-reduced-motion="false"] [data-first-blood-banner-panel] {
          animation:
            ad-first-blood-banner-slam 420ms cubic-bezier(0.16, 1, 0.3, 1) both,
            ad-first-blood-banner-glow 900ms ease-out 200ms both,
            ad-first-blood-banner-breathe 2200ms ease-in-out 1100ms infinite;
        }
        [data-testid="first-blood-banner"][data-reduced-motion="false"] [data-first-blood-banner-body] > * {
          animation: ad-first-blood-banner-rise 460ms cubic-bezier(0.16, 1, 0.3, 1) both;
        }
        [data-testid="first-blood-banner"][data-reduced-motion="false"] [data-first-blood-banner-body] > :nth-child(1) { animation-delay: 220ms; }
        [data-testid="first-blood-banner"][data-reduced-motion="false"] [data-first-blood-banner-body] > :nth-child(2) { animation-delay: 320ms; }
        [data-testid="first-blood-banner"][data-reduced-motion="false"] [data-first-blood-banner-body] > :nth-child(3) { animation-delay: 400ms; }
        [data-testid="first-blood-banner"][data-reduced-motion="false"] [data-first-blood-banner-body] > :nth-child(n+4) { animation-delay: 480ms; }
        [data-first-blood-banner-shine] {
          background: linear-gradient(90deg, transparent, color-mix(in srgb, var(--signal-negative) 22%, transparent), transparent);
          animation: ad-first-blood-banner-shine 900ms cubic-bezier(0.4, 0, 0.2, 1) 520ms both;
        }
        [data-testid="first-blood-banner"][data-reduced-motion="false"] [data-first-blood-banner-panel] h3 {
          animation: ad-first-blood-banner-heading 520ms cubic-bezier(0.16, 1, 0.3, 1) 80ms both;
        }
        [data-first-blood-banner-slash] {
          transform-origin: left center;
          animation: ad-first-blood-banner-slash 460ms cubic-bezier(0.7, 0, 0.3, 1) both;
        }
        [data-testid="first-blood-banner"][data-banner-phase="exit"][data-reduced-motion="false"] [data-first-blood-banner-panel] {
          animation: ad-first-blood-banner-exit ${FIRST_BLOOD_BANNER_EXIT_MS}ms cubic-bezier(0.7, 0, 0.84, 0) both;
        }
        @media (prefers-reduced-motion: reduce) {
          [data-first-blood-banner-panel],
          [data-first-blood-banner-panel] h3,
          [data-first-blood-banner-body] > *,
          [data-first-blood-banner-shine],
          [data-first-blood-banner-slash] { animation: none !important; }
          [data-first-blood-banner-slash],
          [data-first-blood-banner-shine] { display: none; }
        }
      `}</style>
    </div>
  );
}

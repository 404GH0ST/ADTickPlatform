'use client';

import { useId, type ReactElement } from 'react';

import { FIRST_BLOOD_BANNER_EXIT_MS } from '@/lib/first-blood';
import { cn } from '@/lib/utils';
import type { CyberAttackEvent } from './cyber-attack-map';

export function FirstBloodBanner({
  attack,
  leaving,
  reducedMotion,
}: {
  attack: CyberAttackEvent;
  leaving: boolean;
  reducedMotion: boolean;
}): ReactElement {
  const headingId = useId();
  const dense = attack.attacker.length + attack.service.length > 100;
  return (
    <div
      role="status"
      aria-atomic="true"
      aria-labelledby={headingId}
      data-testid="first-blood-banner"
      data-attack-id={attack.id}
      data-banner-phase={leaving ? 'exit' : 'hold'}
      data-reduced-motion={reducedMotion ? 'true' : 'false'}
      className="pointer-events-none absolute inset-x-3 top-1/2 z-20 -translate-y-1/2 sm:inset-x-6"
      style={{ color: 'var(--attack-map-node-label)' }}
    >
      <div
        data-first-blood-banner-panel="true"
        className={cn(
          'mx-auto grid max-w-[46rem] min-w-0 border-y px-4 text-center sm:grid-cols-[minmax(0,0.55fr)_minmax(0,1fr)] sm:items-center sm:gap-8 sm:px-8 sm:py-7 sm:text-left',
          dense ? 'gap-3 py-4' : 'gap-4 py-5',
        )}
        style={{
          backgroundColor: 'var(--attack-map-background)',
          borderColor: 'var(--attack-map-legend-border)',
        }}
      >
        <h3
          id={headingId}
          className={cn(
            'font-semibold leading-tight sm:text-3xl',
            dense ? 'text-xl' : 'text-2xl',
          )}
          style={{ color: 'var(--signal-negative)' }}
        >
          First blood
        </h3>
        <div className={cn('min-w-0', dense ? 'space-y-2' : 'space-y-3')}>
          <p
            data-testid="first-blood-banner-team"
            className={cn(
              'text-balance font-semibold leading-tight [overflow-wrap:anywhere]',
              dense ? 'text-lg sm:text-3xl' : 'text-2xl sm:text-4xl',
            )}
          >
            {attack.attacker}
          </p>
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
            <span aria-hidden="true"> · </span>Tick #{attack.tick}
          </p>
        </div>
      </div>
      <style>{`
        @keyframes ad-first-blood-banner-enter {
          from { opacity: 0; clip-path: inset(0 6%); }
          to { opacity: 1; clip-path: inset(0); }
        }
        @keyframes ad-first-blood-banner-exit {
          from { opacity: 1; }
          to { opacity: 0; }
        }
        [data-testid="first-blood-banner"][data-reduced-motion="false"] [data-first-blood-banner-panel] {
          animation: ad-first-blood-banner-enter 260ms cubic-bezier(0.16, 1, 0.3, 1) both;
        }
        [data-testid="first-blood-banner"][data-banner-phase="exit"][data-reduced-motion="false"] [data-first-blood-banner-panel] {
          animation: ad-first-blood-banner-exit ${FIRST_BLOOD_BANNER_EXIT_MS}ms ease both;
        }
        @media (prefers-reduced-motion: reduce) {
          [data-first-blood-banner-panel] { animation: none !important; }
        }
      `}</style>
    </div>
  );
}

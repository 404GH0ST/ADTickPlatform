'use client';

import type { ReactElement, ReactNode } from 'react';

import { cn } from '@/lib/utils';

type InfoPanelTone = 'muted' | 'surface' | 'warning';
type InfoPanelLayout = 'stack' | 'grid';

type InfoPanelProps = {
  children: ReactNode;
  className?: string;
  compact?: boolean;
  layout?: InfoPanelLayout;
  tone?: InfoPanelTone;
};

type InfoLineProps = {
  className?: string;
  label: string;
  value: ReactNode;
  valueClassName?: string;
};

const toneClassName: Record<InfoPanelTone, string> = {
  muted: 'border border-border/70 bg-muted/20 text-muted-foreground',
  surface: 'border border-border/70 bg-background text-muted-foreground',
  warning: 'border tone-warning',
};

const layoutClassName: Record<InfoPanelLayout, string> = {
  stack: 'space-y-3',
  grid: 'grid gap-3',
};

export function InfoPanel({
  children,
  className,
  compact = false,
  layout = 'stack',
  tone = 'muted',
}: InfoPanelProps): ReactElement {
  return (
    <div
      className={cn(
        layoutClassName[layout],
        compact ? 'p-3' : 'p-4',
        'rounded-md text-sm',
        toneClassName[tone],
        className,
      )}
    >
      {children}
    </div>
  );
}

export function InfoLine({
  className,
  label,
  value,
  valueClassName,
}: InfoLineProps): ReactElement {
  return (
    <p className={className}>
      {label}:{' '}
      <span className={cn('text-foreground', valueClassName)}>
        {value}
      </span>
    </p>
  );
}

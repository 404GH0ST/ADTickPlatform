'use client';

import type { ReactElement } from 'react';

type StatusBannerVariant = 'error' | 'success' | 'warning' | 'info';

type StatusBannerProps = {
  message: string;
  variant: StatusBannerVariant;
};

const bannerTone: Record<StatusBannerVariant, string> = {
  error: 'tone-danger',
  success: 'tone-success',
  warning: 'tone-warning',
  info: 'tone-info',
};

export function StatusBanner({
  message,
  variant,
}: StatusBannerProps): ReactElement {
  return (
    <div className={`rounded-sm border px-3 py-2 text-sm ${bannerTone[variant]}`}>
      {message}
    </div>
  );
}

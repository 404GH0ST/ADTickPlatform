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
    <div
      role="status"
      className={`rounded-sm border px-3.5 py-2.5 text-sm leading-5 ${bannerTone[variant]}`}
    >
      {message}
    </div>
  );
}

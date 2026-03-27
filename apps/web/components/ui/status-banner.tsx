'use client';

import type { ReactElement } from 'react';

type StatusBannerVariant = 'error' | 'success' | 'warning';

type StatusBannerProps = {
  message: string;
  variant: StatusBannerVariant;
};

const bannerTone: Record<StatusBannerVariant, string> = {
  error: 'tone-danger',
  success: 'tone-success',
  warning: 'tone-warning',
};

export function StatusBanner({
  message,
  variant,
}: StatusBannerProps): ReactElement {
  return (
    <div className={`rounded-md border p-3 text-sm ${bannerTone[variant]}`}>
      {message}
    </div>
  );
}

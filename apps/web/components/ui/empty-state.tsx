'use client';

import type { ReactElement } from 'react';

import { TableCell, TableRow } from '@/components/ui/table';

type EmptyStateTextProps = {
  message: string;
};

type EmptyTableRowProps = {
  colSpan: number;
  message: string;
};

export function EmptyStateText({
  message,
}: EmptyStateTextProps): ReactElement {
  return <p className="text-sm text-muted-foreground">{message}</p>;
}

export function EmptyTableRow({
  colSpan,
  message,
}: EmptyTableRowProps): ReactElement {
  return (
    <TableRow>
      <TableCell colSpan={colSpan} className="text-center text-sm text-muted-foreground">
        {message}
      </TableCell>
    </TableRow>
  );
}

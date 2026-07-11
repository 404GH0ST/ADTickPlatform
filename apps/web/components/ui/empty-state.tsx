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
    <TableRow className="hover:bg-transparent">
      <TableCell
        colSpan={colSpan}
        className="h-32 bg-muted/10 px-4 text-center text-sm text-muted-foreground"
      >
        {message}
      </TableCell>
    </TableRow>
  );
}

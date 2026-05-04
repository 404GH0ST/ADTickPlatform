import { Table, TableHeader, TableRow, TableHead } from "@/components/ui/table";
import { ReactNode } from "react";

export function AdminTable({ children }: { children: ReactNode }) {
  return <Table className="[&_td]:px-3 [&_td]:py-2.5 [&_th]:h-10 [&_th]:px-3">{children}</Table>;
}

export function AdminTableHeader({ columns }: { columns: { label: string; className?: string }[] }) {
  return (
    <TableHeader>
      <TableRow>
        {columns.map((col, i) => (
          <TableHead key={i} className={col.className}>
            {col.label}
          </TableHead>
        ))}
      </TableRow>
    </TableHeader>
  );
}

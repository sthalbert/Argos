import type { ReactNode } from 'react';
import { KV, SectionTitle } from '../../components';

// NetworkingCard renders the "Networking" block of a compute-detail page.
// Both Node and VirtualMachine pages use it so adding a column in one
// place benefits both. The shape is intentionally minimal — pages pass
// labelled rows; specialised sub-tables (NICs, security groups) live as
// children.

export interface NetworkingRow {
  label: string;
  value: ReactNode;
}

export function NetworkingCard({
  rows,
  children,
}: {
  rows: NetworkingRow[];
  children?: ReactNode;
}) {
  return (
    <>
      <SectionTitle>Networking</SectionTitle>
      <dl className="kv-list">
        {rows.map((r) => (
          <KV key={r.label} k={r.label} v={r.value} />
        ))}
      </dl>
      {children}
    </>
  );
}

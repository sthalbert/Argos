import type { ReactNode } from 'react';
import { KV } from '../../components';
import { SectionTitle } from '../../components';

// IdentityCard renders a kv-list inside a SectionTitled "Identity" card.
// Used by the Node detail page and the VirtualMachine detail page so the
// header shape stays consistent across compute kinds.

export interface IdentityRow {
  label: string;
  value: ReactNode;
}

export function IdentityCard({ rows, title = 'Identity' }: { rows: IdentityRow[]; title?: string }) {
  return (
    <>
      <SectionTitle>{title}</SectionTitle>
      <dl className="kv-list">
        {rows.map((r) => (
          <KV key={r.label} k={r.label} v={r.value} />
        ))}
      </dl>
    </>
  );
}

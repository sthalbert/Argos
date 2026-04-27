import { Dash, SectionTitle } from '../../components';

// CapacityCard renders a capacity / allocatable table for compute kinds.
// Nodes carry both columns; VMs only carry capacity (no allocatable from
// providers), so callers can pass the allocatable column as undefined.

export interface CapacityRow {
  dimension: string;
  capacity?: string | null;
  allocatable?: string | null;
}

export function CapacityCard({
  rows,
  showAllocatable = true,
  emptyMessage,
}: {
  rows: CapacityRow[];
  showAllocatable?: boolean;
  emptyMessage?: string;
}) {
  const hasAny = rows.some(
    (r) => (r.capacity && r.capacity !== '') || (r.allocatable && r.allocatable !== ''),
  );
  return (
    <>
      <SectionTitle>{showAllocatable ? 'Resources' : 'Capacity'}</SectionTitle>
      {!hasAny && emptyMessage ? (
        <p className="muted empty">{emptyMessage}</p>
      ) : (
        <table className="entities">
          <thead>
            <tr>
              <th>Dimension</th>
              <th>Capacity</th>
              {showAllocatable && <th>Allocatable</th>}
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => (
              <tr key={r.dimension}>
                <td>{r.dimension}</td>
                <td>{r.capacity ? <code>{r.capacity}</code> : <Dash />}</td>
                {showAllocatable && (
                  <td>{r.allocatable ? <code>{r.allocatable}</code> : <Dash />}</td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  );
}

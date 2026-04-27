import { Labels, SectionTitle } from '../../components';

// LabelsCard renders a labels block (Kubernetes labels, cloud tags, …)
// as a SectionTitled chip list. The label below the section heading
// adapts to whether we're rendering Kubernetes labels or cloud-provider
// tags so users immediately know which they're looking at.

export function LabelsCard({
  labels,
  title = 'Labels',
}: {
  labels?: Record<string, string> | null;
  title?: string;
}) {
  return (
    <>
      <SectionTitle>{title}</SectionTitle>
      <Labels labels={labels} />
    </>
  );
}

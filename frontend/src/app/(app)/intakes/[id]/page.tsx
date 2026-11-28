import { Suspense } from "react";

import { IntakeDetailView } from "@/components/intake-detail";
import { Spinner } from "@/components/ui";

export default function IntakeDetailPage({ params }: { params: Promise<{ id: string }> }) {
  return (
    <Suspense fallback={<Spinner />}>
      <Detail params={params} />
    </Suspense>
  );
}

async function Detail({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <IntakeDetailView id={id} />;
}

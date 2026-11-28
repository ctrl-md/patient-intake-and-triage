import { Suspense } from "react";

import { IntakeChat } from "@/components/chat";
import { Spinner } from "@/components/ui";

// The patient opens this from the link the clinic sends. No login.
export default function IntakePage({ params }: { params: Promise<{ token: string }> }) {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-[60vh] items-center justify-center">
          <Spinner />
        </div>
      }
    >
      <Chat params={params} />
    </Suspense>
  );
}

async function Chat({ params }: { params: Promise<{ token: string }> }) {
  const { token } = await params;
  return <IntakeChat linkToken={token} />;
}

import type { ReactNode } from "react";

import { AuthGate } from "@/components/auth-gate";
import { AppHeader } from "@/components/app-header";

// Everything inside the (app) folder needs a login.
export default function AppLayout({ children }: { children: ReactNode }) {
  return (
    <AuthGate>
      <AppHeader />
      <main className="mx-auto w-full max-w-5xl px-4 py-8">{children}</main>
    </AuthGate>
  );
}

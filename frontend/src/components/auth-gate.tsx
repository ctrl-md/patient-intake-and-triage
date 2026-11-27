"use client";

import { useRouter } from "next/navigation";
import { useEffect, type ReactNode } from "react";

import { useAuth } from "@/components/auth-provider";
import { Spinner } from "@/components/ui";

/** Shows its children only to logged-in staff. Everyone else goes to /login. */
export function AuthGate({ children }: { children: ReactNode }) {
  const { state } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (state.status === "anon") router.replace("/login");
  }, [state.status, router]);

  if (state.status !== "authed") {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Spinner label="Checking your login..." />
      </div>
    );
  }
  return <>{children}</>;
}

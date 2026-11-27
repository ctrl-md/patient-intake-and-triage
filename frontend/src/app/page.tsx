"use client";

import { useRouter } from "next/navigation";
import { useEffect } from "react";

import { useAuth } from "@/components/auth-provider";
import { Spinner } from "@/components/ui";

// The home page has nothing of its own. It sends people where they belong.
export default function Home() {
  const { state } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (state.status === "authed") router.replace("/dashboard");
    if (state.status === "anon") router.replace("/login");
  }, [state.status, router]);

  return (
    <div className="flex min-h-screen items-center justify-center">
      <Spinner />
    </div>
  );
}

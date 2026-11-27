"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";

import { AuthCard } from "@/components/auth-card";
import { useAuth } from "@/components/auth-provider";
import { Alert, Button, Field } from "@/components/ui";
import { messageOf } from "@/lib/errors";

export default function LoginPage() {
  const { state, login } = useAuth();
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  // Already logged in? Skip the form.
  useEffect(() => {
    if (state.status === "authed") router.replace("/dashboard");
  }, [state.status, router]);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      await login(email.trim(), password);
      router.replace("/dashboard");
    } catch (err) {
      setError(messageOf(err));
      setBusy(false);
    }
  }

  return (
    <AuthCard title="Staff login" subtitle="Sign in to see your triage queue.">
      <form onSubmit={onSubmit} className="space-y-4">
        {error ? <Alert>{error}</Alert> : null}
        <Field
          label="Email"
          name="email"
          type="email"
          autoComplete="email"
          required
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          disabled={busy}
        />
        <Field
          label="Password"
          name="password"
          type="password"
          autoComplete="current-password"
          required
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          disabled={busy}
        />
        <Button type="submit" disabled={busy} className="w-full">
          {busy ? "Signing in..." : "Sign in"}
        </Button>
      </form>
      <p className="text-center text-sm text-slate-500">
        New here?{" "}
        <Link href="/register" className="font-medium text-teal-700 hover:underline">
          Create a staff account
        </Link>
      </p>
    </AuthCard>
  );
}

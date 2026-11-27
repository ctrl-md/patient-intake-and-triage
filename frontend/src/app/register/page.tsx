"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";

import { AuthCard } from "@/components/auth-card";
import { useAuth } from "@/components/auth-provider";
import { Alert, Button, Field } from "@/components/ui";
import { messageOf } from "@/lib/errors";

export default function RegisterPage() {
  const { state, register } = useAuth();
  const router = useRouter();
  const [fullName, setFullName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (state.status === "authed") router.replace("/dashboard");
  }, [state.status, router]);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);

    // The API checks these too. Checking here just saves a round trip.
    if (password.length < 8) {
      setError("Password must be at least 8 characters.");
      return;
    }

    setBusy(true);
    try {
      await register({ full_name: fullName.trim(), email: email.trim(), password });
      router.replace("/dashboard");
    } catch (err) {
      setError(messageOf(err));
      setBusy(false);
    }
  }

  return (
    <AuthCard title="Create a staff account" subtitle="New accounts start as nurse.">
      <form onSubmit={onSubmit} className="space-y-4">
        {error ? <Alert>{error}</Alert> : null}
        <Field
          label="Full name"
          name="full_name"
          autoComplete="name"
          required
          maxLength={100}
          value={fullName}
          onChange={(e) => setFullName(e.target.value)}
          disabled={busy}
        />
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
          autoComplete="new-password"
          required
          minLength={8}
          maxLength={72}
          hint="8 to 72 characters."
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          disabled={busy}
        />
        <Button type="submit" disabled={busy} className="w-full">
          {busy ? "Creating account..." : "Create account"}
        </Button>
      </form>
      <p className="text-center text-sm text-slate-500">
        Already have an account?{" "}
        <Link href="/login" className="font-medium text-teal-700 hover:underline">
          Sign in
        </Link>
      </p>
    </AuthCard>
  );
}

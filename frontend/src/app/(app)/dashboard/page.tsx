"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { useAuthed } from "@/components/auth-provider";
import { UrgencyBadge } from "@/components/urgency-badge";
import { Alert, Button, Spinner } from "@/components/ui";
import { api } from "@/lib/api";
import { messageOf } from "@/lib/errors";
import type { CreateIntakeResponse, Intake, IntakeStatus } from "@/lib/types";

const STATUS_STYLE: Record<IntakeStatus, string> = {
  pending: "bg-slate-100 text-slate-700",
  in_progress: "bg-amber-100 text-amber-800",
  completed: "bg-emerald-100 text-emerald-800",
  expired: "bg-red-100 text-red-700",
};

const STATUS_LABEL: Record<IntakeStatus, string> = {
  pending: "Waiting",
  in_progress: "In progress",
  completed: "Completed",
  expired: "Expired",
};

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString(undefined, {
    day: "numeric",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export default function DashboardPage() {
  const { token, staff } = useAuthed();

  const [intakes, setIntakes] = useState<Intake[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [created, setCreated] = useState<CreateIntakeResponse | null>(null);
  const [copied, setCopied] = useState(false);

  // Bump this to load the list again.
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    api
      .listIntakes(token, { limit: 50, signal: controller.signal })
      .then((res) => {
        setIntakes(res.intakes);
        setError(null);
      })
      .catch((err) => {
        if (!controller.signal.aborted) setError(messageOf(err));
      });
    return () => controller.abort();
  }, [token, reloadKey]);

  async function onCreate() {
    setCreating(true);
    setError(null);
    setCopied(false);
    try {
      setCreated(await api.createIntake(token));
      setReloadKey((k) => k + 1);
    } catch (err) {
      setError(messageOf(err));
    } finally {
      setCreating(false);
    }
  }

  async function onCopy(link: string) {
    try {
      await navigator.clipboard.writeText(link);
      setCopied(true);
    } catch {
      setError("Could not copy. Select the link and copy it by hand.");
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold text-slate-900">Dashboard</h1>
          <p className="text-sm text-slate-500">
            Welcome, {staff.full_name}. Recent patient intakes are listed below.
          </p>
        </div>
        <Button onClick={onCreate} disabled={creating}>
          {creating ? "Creating..." : "New intake"}
        </Button>
      </div>

      {error ? <Alert>{error}</Alert> : null}

      {created ? (
        <section className="space-y-2 rounded-lg border border-teal-200 bg-teal-50 p-4">
          <p className="text-sm font-medium text-teal-900">Intake link ready. Send it to the patient.</p>
          <div className="flex flex-wrap items-center gap-2">
            <input
              readOnly
              value={created.link}
              aria-label="Patient intake link"
              onFocus={(e) => e.currentTarget.select()}
              className="min-w-0 flex-1 rounded-md border border-teal-200 bg-white px-3 py-2 font-mono text-xs"
            />
            <Button variant="ghost" onClick={() => onCopy(created.link)}>
              {copied ? "Copied" : "Copy"}
            </Button>
          </div>
          <p className="text-xs text-teal-800">
            This link is shown only now. It works until {formatDate(created.intake.expires_at)}.
          </p>
        </section>
      ) : null}

      <section className="overflow-hidden rounded-lg border border-slate-200 bg-white">
        {intakes === null && !error ? (
          <div className="p-6">
            <Spinner label="Loading intakes..." />
          </div>
        ) : intakes && intakes.length === 0 ? (
          <p className="p-6 text-sm text-slate-500">
            No intakes yet. Click &ldquo;New intake&rdquo; to make a link for a patient.
          </p>
        ) : intakes ? (
          <table className="w-full text-left text-sm">
            <thead className="border-b border-slate-200 bg-slate-50 text-xs uppercase tracking-wide text-slate-500">
              <tr>
                <th className="px-4 py-2 font-medium">Patient</th>
                <th className="px-4 py-2 font-medium">Urgency</th>
                <th className="px-4 py-2 font-medium">Status</th>
                <th className="px-4 py-2 font-medium">Messages</th>
                <th className="px-4 py-2 font-medium">Created</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {intakes.map((i) => (
                <tr key={i.id}>
                  <td className="px-4 py-3 text-slate-900">
                    <Link href={`/intakes/${i.id}`} className="hover:text-teal-700 hover:underline">
                      {i.patient_name ?? <span className="text-slate-400">Not named yet</span>}
                    </Link>
                  </td>
                  <td className="px-4 py-3">
                    <UrgencyBadge urgency={i.urgency} />
                  </td>
                  <td className="px-4 py-3">
                    <span
                      className={`rounded-full px-2 py-0.5 text-xs font-medium ${STATUS_STYLE[i.status]}`}
                    >
                      {STATUS_LABEL[i.status]}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-slate-600">{i.message_count}</td>
                  <td className="px-4 py-3 text-slate-600">{formatDate(i.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : null}
      </section>
    </div>
  );
}

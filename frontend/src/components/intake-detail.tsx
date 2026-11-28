"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { useAuthed } from "@/components/auth-provider";
import { UrgencyBadge } from "@/components/urgency-badge";
import { Alert, Button, Spinner } from "@/components/ui";
import { api } from "@/lib/api";
import { messageOf } from "@/lib/errors";
import type { IntakeDetail, Triage } from "@/lib/types";

const ROLE_LABEL = { patient: "Patient", assistant: "Assistant", system: "System" } as const;

export function IntakeDetailView({ id }: { id: string }) {
  const { token } = useAuthed();
  const [detail, setDetail] = useState<IntakeDetail | null>(null);
  const [triage, setTriage] = useState<Triage | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [running, setRunning] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    Promise.all([api.getIntake(token, id, controller.signal), api.getTriage(token, id, controller.signal)])
      .then(([d, t]) => {
        setDetail(d);
        setTriage(t);
        setError(null);
      })
      .catch((err) => {
        if (!controller.signal.aborted) setError(messageOf(err));
      });
    return () => controller.abort();
  }, [token, id]);

  async function onRun() {
    setRunning(true);
    setError(null);
    try {
      setTriage(await api.runTriage(token, id));
    } catch (err) {
      setError(messageOf(err));
    } finally {
      setRunning(false);
    }
  }

  if (!detail) {
    return error ? <Alert>{error}</Alert> : <Spinner label="Loading intake..." />;
  }

  const { intake, messages } = detail;
  const completed = intake.status === "completed";

  return (
    <div className="space-y-6">
      <div>
        <Link href="/dashboard" className="text-sm text-teal-700 hover:underline">
          &larr; Back to dashboard
        </Link>
        <div className="mt-2 flex flex-wrap items-center gap-3">
          <h1 className="text-xl font-semibold text-slate-900">
            {intake.patient_name ?? "Unnamed patient"}
          </h1>
          <UrgencyBadge urgency={triage?.staff_urgency ?? triage?.final_urgency} />
        </div>
        <p className="text-sm text-slate-500">Status: {intake.status.replace("_", " ")}</p>
      </div>

      {error ? <Alert>{error}</Alert> : null}

      <section className="space-y-3 rounded-lg border border-slate-200 bg-white p-4">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-semibold text-slate-900">Triage</h2>
          {completed ? (
            <Button variant="ghost" onClick={onRun} disabled={running}>
              {running ? "Running..." : triage ? "Run again" : "Run triage"}
            </Button>
          ) : null}
        </div>

        {!triage ? (
          <p className="text-sm text-slate-500">
            {completed
              ? "No triage result yet. It runs a few seconds after the patient finishes. Reload, or run it now."
              : "Triage runs when the patient finishes the chat."}
          </p>
        ) : (
          <TriageView triage={triage} />
        )}
      </section>

      <section className="space-y-3 rounded-lg border border-slate-200 bg-white p-4">
        <h2 className="text-sm font-semibold text-slate-900">Conversation</h2>
        {messages.length === 0 ? (
          <p className="text-sm text-slate-500">No messages yet.</p>
        ) : (
          <ul className="space-y-2">
            {messages.map((m, i) => (
              <li key={m.id ?? i} className="text-sm">
                <span className="font-medium text-slate-700">{ROLE_LABEL[m.role]}: </span>
                <span className="whitespace-pre-wrap text-slate-900">{m.content}</span>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}

function TriageView({ triage }: { triage: Triage }) {
  return (
    <div className="space-y-4 text-sm">
      <dl className="grid grid-cols-3 gap-3">
        <Stat label="Final" urgency={triage.final_urgency} />
        <Stat label="Rules" urgency={triage.rules_urgency} />
        <Stat label="AI" urgency={triage.ai_urgency} />
      </dl>

      {triage.ai_urgency === null ? (
        <Alert>
          The AI could not read this case, so it was marked at least urgent to be safe. Please read the
          conversation yourself.
        </Alert>
      ) : (
        <div className="space-y-1">
          <h3 className="font-medium text-slate-900">Summary</h3>
          <p className="whitespace-pre-wrap text-slate-700">{triage.ai_summary}</p>
          <p className="text-xs text-slate-500">Why: {triage.ai_reasoning}</p>
        </div>
      )}

      <div className="space-y-1">
        <h3 className="font-medium text-slate-900">Red flags</h3>
        {triage.red_flags.length === 0 ? (
          <p className="text-slate-500">None found by the rules.</p>
        ) : (
          <ul className="space-y-1">
            {triage.red_flags.map((f) => (
              <li key={f.code} className="flex flex-wrap items-center gap-2">
                <UrgencyBadge urgency={f.urgency} />
                <span className="text-slate-900">{f.label}</span>
                <span className="text-xs text-slate-500">&ldquo;{f.evidence}&rdquo;</span>
              </li>
            ))}
          </ul>
        )}
      </div>

      <p className="text-xs text-slate-400">
        Final = the higher of rules and AI. The AI can raise urgency, never lower it. Demo only.
      </p>
    </div>
  );
}

function Stat({ label, urgency }: { label: string; urgency: Triage["final_urgency"] | null }) {
  return (
    <div className="rounded-md border border-slate-200 p-2">
      <dt className="text-xs uppercase tracking-wide text-slate-500">{label}</dt>
      <dd className="mt-1">
        {urgency ? <UrgencyBadge urgency={urgency} /> : <span className="text-xs text-slate-400">No answer</span>}
      </dd>
    </div>
  );
}

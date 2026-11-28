"use client";

import { useEffect, useRef, useState, type FormEvent } from "react";

import { Alert, Button, Spinner } from "@/components/ui";
import { api, ApiError } from "@/lib/api";
import { messageOf } from "@/lib/errors";
import type { ChatMessage, IntakeStatus } from "@/lib/types";

const MAX_CHARS = 2000;

type Load =
  | { kind: "loading" }
  | { kind: "ready" }
  | { kind: "problem"; title: string; text: string };

const WELCOME: ChatMessage = {
  role: "assistant",
  content:
    "Hello. I am the clinic's intake assistant. I will ask a few questions before you see the nurse. What is the main problem that brought you in?",
  created_at: "",
};

export function IntakeChat({ linkToken }: { linkToken: string }) {
  const [load, setLoad] = useState<Load>({ kind: "loading" });
  const [status, setStatus] = useState<IntakeStatus>("pending");
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const [finishing, setFinishing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [emergency, setEmergency] = useState(false);
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const controller = new AbortController();
    api
      .getPublicIntake(linkToken, controller.signal)
      .then((res) => {
        setStatus(res.status);
        setMessages(res.messages);
        setLoad({ kind: "ready" });
      })
      .catch((err) => {
        if (controller.signal.aborted) return;
        if (err instanceof ApiError && err.status === 410) {
          setLoad({
            kind: "problem",
            title: "This link has expired",
            text: "Please ask the clinic for a new link.",
          });
        } else if (err instanceof ApiError && err.status === 404) {
          setLoad({
            kind: "problem",
            title: "Link not found",
            text: "Check that you copied the whole link, or ask the clinic to send it again.",
          });
        } else {
          setLoad({ kind: "problem", title: "Could not open the chat", text: messageOf(err) });
        }
      });
    return () => controller.abort();
  }, [linkToken]);

  useEffect(() => {
    bottomRef.current?.scrollIntoView?.({ behavior: "smooth", block: "end" });
  }, [messages, sending]);

  async function onSend(e: FormEvent) {
    e.preventDefault();
    const content = draft.trim();
    if (!content || sending) return;

    setSending(true);
    setError(null);
    try {
      const res = await api.sendMessage(linkToken, content);
      setDraft("");
      setMessages((prev) => [...prev, res.message, ...(res.reply ? [res.reply] : [])]);
      setStatus((s) => (s === "pending" ? "in_progress" : s));
      if (res.emergency) setEmergency(true);
    } catch (err) {
      // The draft stays in the box so nothing is lost.
      setError(messageOf(err));
    } finally {
      setSending(false);
    }
  }

  async function onFinish() {
    if (finishing) return;
    setFinishing(true);
    setError(null);
    try {
      const res = await api.completeIntake(linkToken);
      setStatus(res.status);
    } catch (err) {
      setError(messageOf(err));
    } finally {
      setFinishing(false);
    }
  }

  if (load.kind === "loading") {
    return (
      <div className="flex min-h-[60vh] items-center justify-center">
        <Spinner label="Opening your chat..." />
      </div>
    );
  }

  if (load.kind === "problem") {
    return (
      <div className="mx-auto max-w-md space-y-2 py-16 text-center">
        <h1 className="text-lg font-semibold text-slate-900">{load.title}</h1>
        <p className="text-sm text-slate-600">{load.text}</p>
      </div>
    );
  }

  const done = status === "completed";
  const shown = messages.length === 0 ? [WELCOME] : messages;
  const hasPatientMessage = messages.some((m) => m.role === "patient");

  return (
    <div className="mx-auto flex h-[100dvh] w-full max-w-2xl flex-col px-4 py-4">
      <header className="pb-3">
        <h1 className="text-lg font-semibold text-slate-900">Triagely intake</h1>
        <p className="text-xs text-slate-500">
          Tell us what is wrong. A nurse will read this. Demo project, not for real medical use.
        </p>
      </header>

      {emergency ? (
        <div
          role="alert"
          className="mb-3 rounded-md border border-red-300 bg-red-50 px-3 py-2 text-sm font-medium text-red-900"
        >
          This may be an emergency. Call your local emergency number now or go to the nearest
          emergency room. Do not wait for the clinic.
        </div>
      ) : null}

      <ol
        aria-label="Conversation"
        className="flex-1 space-y-3 overflow-y-auto rounded-lg border border-slate-200 bg-white p-4"
      >
        {shown.map((m, i) => (
          <li key={m.id ?? `${i}-${m.created_at}`} className={m.role === "patient" ? "flex justify-end" : "flex"}>
            <div
              className={`max-w-[85%] whitespace-pre-wrap rounded-2xl px-3 py-2 text-sm ${
                m.role === "patient" ? "bg-teal-700 text-white" : "bg-slate-100 text-slate-900"
              }`}
            >
              {m.content}
            </div>
          </li>
        ))}
        {sending ? (
          <li className="flex">
            <div className="rounded-2xl bg-slate-100 px-3 py-2 text-sm text-slate-500">Typing...</div>
          </li>
        ) : null}
        <div ref={bottomRef} />
      </ol>

      {error ? (
        <div className="mt-3">
          <Alert>{error}</Alert>
        </div>
      ) : null}

      {done ? (
        <p className="mt-3 rounded-md border border-emerald-200 bg-emerald-50 px-3 py-3 text-sm text-emerald-900">
          Thank you. Your answers were sent to the clinic. You can close this page.
        </p>
      ) : (
        <form onSubmit={onSend} className="mt-3 space-y-2">
          <label htmlFor="message" className="sr-only">
            Your message
          </label>
          <textarea
            id="message"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault();
                e.currentTarget.form?.requestSubmit();
              }
            }}
            maxLength={MAX_CHARS}
            rows={2}
            placeholder="Type here..."
            disabled={sending}
            className="block w-full resize-none rounded-md border border-slate-300 bg-white px-3 py-2 text-sm shadow-sm outline-none focus:border-teal-600 focus:ring-2 focus:ring-teal-600/20 disabled:bg-slate-100"
          />
          <div className="flex items-center justify-between gap-2">
            <Button type="button" variant="ghost" onClick={onFinish} disabled={!hasPatientMessage || finishing || sending}>
              {finishing ? "Sending..." : "Finish"}
            </Button>
            <Button type="submit" disabled={sending || draft.trim() === ""}>
              {sending ? "Sending..." : "Send"}
            </Button>
          </div>
        </form>
      )}
    </div>
  );
}

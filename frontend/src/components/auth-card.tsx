import type { ReactNode } from "react";

/** The centered card used by the login and register pages. */
export function AuthCard({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle: string;
  children: ReactNode;
}) {
  return (
    <main className="flex min-h-screen items-center justify-center px-4 py-10">
      <div className="w-full max-w-sm space-y-6">
        <div className="text-center">
          <p className="text-2xl font-semibold tracking-tight text-teal-800">Triagely</p>
          <p className="mt-1 text-sm text-slate-500">Patient intake and triage</p>
        </div>
        <div className="space-y-5 rounded-xl border border-slate-200 bg-white p-6 shadow-sm">
          <div>
            <h1 className="text-lg font-semibold text-slate-900">{title}</h1>
            <p className="text-sm text-slate-500">{subtitle}</p>
          </div>
          {children}
        </div>
        <p className="text-center text-xs text-slate-400">
          Demo project. Not a medical device. Use fake data only.
        </p>
      </div>
    </main>
  );
}

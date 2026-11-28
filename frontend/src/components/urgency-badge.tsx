import type { Urgency } from "@/lib/types";

const STYLE: Record<Urgency, string> = {
  routine: "bg-emerald-100 text-emerald-800",
  urgent: "bg-amber-100 text-amber-900",
  emergency: "bg-red-600 text-white",
};

const LABEL: Record<Urgency, string> = {
  routine: "Routine",
  urgent: "Urgent",
  emergency: "Emergency",
};

export function UrgencyBadge({ urgency }: { urgency: Urgency | null | undefined }) {
  if (!urgency) return <span className="text-xs text-slate-400">Not triaged</span>;
  return (
    <span className={`rounded-full px-2 py-0.5 text-xs font-semibold ${STYLE[urgency]}`}>
      {LABEL[urgency]}
    </span>
  );
}

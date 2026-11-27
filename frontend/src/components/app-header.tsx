"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";

import { useAuthed } from "@/components/auth-provider";
import { Button } from "@/components/ui";

export function AppHeader() {
  const { staff, logout } = useAuthed();
  const router = useRouter();

  function onLogout() {
    logout();
    router.replace("/login");
  }

  return (
    <header className="border-b border-slate-200 bg-white">
      <div className="mx-auto flex w-full max-w-5xl items-center justify-between px-4 py-3">
        <div className="flex items-center gap-6">
          <Link href="/dashboard" className="text-lg font-semibold tracking-tight text-teal-800">
            Triagely
          </Link>
          <nav className="flex items-center gap-4 text-sm text-slate-600">
            <Link href="/dashboard" className="hover:text-slate-900">
              Dashboard
            </Link>
          </nav>
        </div>
        <div className="flex items-center gap-3">
          <div className="text-right text-sm leading-tight">
            <p className="font-medium text-slate-900">{staff.full_name}</p>
            <p className="text-xs capitalize text-slate-500">{staff.role}</p>
          </div>
          <Button variant="ghost" onClick={onLogout}>
            Log out
          </Button>
        </div>
      </div>
    </header>
  );
}

"use client";
// Phase 68.2 coordinator attendance: backend authorizes all verification requests.
import { use, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { ArrowLeft, CheckCircle2, ClipboardCheck, ShieldCheck } from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import { Alert } from "@/components/ui/Alert";
import { Spinner } from "@/components/ui/Spinner";
import type { EventInstance } from "@/lib/types";

type VerifyResponse = { message?: string; registration?: { status?: string }; claim?: { id?: string } };

export default function CoordinatorAttendancePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const router = useRouter();
  const { user, isCoordinator, isLoading } = useAuth();
  const [event, setEvent] = useState<EventInstance | null>(null);
  const [loading, setLoading] = useState(true);
  const [checkinCode, setCheckinCode] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [verifiedCount, setVerifiedCount] = useState(0);
  const [feedback, setFeedback] = useState<{ type: "success" | "error"; message: string } | null>(null);

  useEffect(() => {
    if (isLoading) return;
    if (!user) { router.replace(`/login?redirect=${encodeURIComponent(`/admin/activities/${id}/attendance`)}`); return; }
    if (!isCoordinator) { router.replace("/member"); return; }
    let active = true;
    apiRequest<EventInstance>(`/activities/events/${id}`)
      .then(data => { if (active) setEvent(data); })
      .catch((err: Error) => { if (active) setFeedback({ type: "error", message: err.message || "Could not load event" }); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [id, user, isCoordinator, isLoading, router]);

  async function verify(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (submitting) return;
    const code = checkinCode.trim().toUpperCase();
    if (!code) { setFeedback({ type: "error", message: "Enter the member's check-in code." }); return; }
    setSubmitting(true);
    setFeedback(null);
    try {
      const result = await apiRequest<VerifyResponse>(`/activities/events/${id}/verify-attendance`, {
        method: "POST",
        body: JSON.stringify({ checkin_code: code, is_manual: false }),
      });
      setVerifiedCount(count => count + 1);
      setCheckinCode("");
      setFeedback({ type: "success", message: result.message || "Attendance verified successfully." });
    } catch (err: unknown) {
      setFeedback({ type: "error", message: err instanceof Error ? err.message : "Attendance verification failed." });
    } finally {
      setSubmitting(false);
    }
  }

  if (isLoading || (user && isCoordinator && loading)) return <Spinner text="Loading attendance verification..." />;
  if (!user || !isCoordinator) return null;
  return (
    <main className="max-w-3xl mx-auto px-4 sm:px-6 py-10 space-y-6">
      <Link href="/admin/activities" className="inline-flex items-center gap-2 text-sm text-gray-600 hover:text-emerald-800">
        <ArrowLeft className="w-4 h-4" /> Back to activity management
      </Link>
      <header className="bg-gradient-to-r from-emerald-950 to-emerald-800 text-white rounded-2xl p-6 sm:p-8">
        <div className="flex items-center gap-2 text-emerald-200 text-xs uppercase tracking-wide font-semibold">
          <ShieldCheck className="w-4 h-4" /> Coordinator workspace
        </div>
        <h1 className="text-2xl font-extrabold mt-2">Verify Cleanup Attendance</h1>
        <p className="text-emerald-100 text-sm mt-2">{event?.title || "Cleanup event"}</p>
        {event && <p className="text-emerald-200 text-xs mt-1">{event.state_name} • {event.lga_name} • {new Date(event.start_time).toLocaleString("en-NG", { timeZone: "Africa/Lagos" })}</p>}
      </header>
      {feedback && <Alert type={feedback.type}>{feedback.message}</Alert>}
      {event && <section className="bg-white border border-gray-200 rounded-2xl shadow-sm p-6 space-y-5">
        <div className="flex items-center gap-3">
          <ClipboardCheck className="w-6 h-6 text-emerald-800" />
          <div>
            <h2 className="font-bold text-gray-900">Member check-in</h2>
            <p className="text-xs text-gray-500">Ask the member to show the code from their registration.</p>
          </div>
        </div>
        <form onSubmit={verify} className="space-y-4">
          <div>
            <label htmlFor="checkin-code" className="block text-sm font-semibold text-gray-800 mb-2">Check-in code</label>
            <input id="checkin-code" type="text" autoComplete="off" autoCapitalize="characters" spellCheck={false}
              required maxLength={32} value={checkinCode} onChange={e => setCheckinCode(e.target.value.toUpperCase())}
              placeholder="RT-XXXX-XXXX"
              className="w-full rounded-xl border border-gray-300 px-4 py-3 font-mono text-lg tracking-wider uppercase focus:ring-2 focus:ring-emerald-600 focus:outline-none" />
          </div>
          <button type="submit" disabled={submitting || !checkinCode.trim()}
            className="w-full sm:w-auto px-6 py-3 bg-emerald-800 hover:bg-emerald-900 text-white rounded-xl font-semibold disabled:opacity-50 disabled:cursor-not-allowed">
            {submitting ? "Verifying..." : "Verify Attendance"}
          </button>
        </form>
        <div className="flex items-center gap-2 text-sm text-emerald-900 bg-emerald-50 border border-emerald-100 rounded-xl p-3">
          <CheckCircle2 className="w-5 h-5 shrink-0" /> Successfully verified in this session: <strong>{verifiedCount}</strong>
        </div>
        <p className="text-xs text-gray-500">The backend checks your jurisdiction, the check-in window, duplicate submissions and self-verification. This count covers only successful verifications made on this page since it opened.</p>
      </section>}
    </main>
  );
}

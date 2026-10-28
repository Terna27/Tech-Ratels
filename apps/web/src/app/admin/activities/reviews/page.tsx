"use client";
// Phase 68.3 coordinator reviews: authorization and scope are enforced by the Go backend.
import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { ArrowLeft, ClipboardList, RefreshCw, ShieldCheck } from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import { Alert } from "@/components/ui/Alert";
import { Spinner } from "@/components/ui/Spinner";

type ReviewItem = {
  id: string;
  event_id?: string;
  event_title?: string;
  user_id?: string;
  member_name?: string;
  display_name?: string;
  description?: string;
  evidence_notes?: string;
  status?: string;
  submitted_at?: string;
  appeal_reason?: string;
  appeals?: Array<{ reason?: string; appeal_reason?: string; status?: string }>;
  review_decision?: string;
  review_reason?: string;
};

type ReviewResponse = ReviewItem[] | {
  claims?: ReviewItem[];
  items?: ReviewItem[];
  queue?: ReviewItem[];
  data?: ReviewItem[];
};

function extractItems(result: ReviewResponse): ReviewItem[] {
  if (Array.isArray(result)) return result;
  const items = result.claims ?? result.items ?? result.queue ?? result.data;
  if (!Array.isArray(items)) throw new Error("Unexpected review queue response. Please check the API response format.");
  return items;
}

export default function CoordinatorParticipationReviewsPage() {
  const router = useRouter();
  const { user, isCoordinator, isLoading } = useAuth();
  const [items, setItems] = useState<ReviewItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [decision, setDecision] = useState<"approved" | "rejected">("approved");
  const [reason, setReason] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [feedback, setFeedback] = useState<{ type: "success" | "error"; message: string } | null>(null);

  async function loadQueue(initial = false) {
    if (initial) setLoading(true);
    else setRefreshing(true);
    try {
      const result = await apiRequest<ReviewResponse>("/participations/review-queue");
      setItems(extractItems(result));
    } catch (error: unknown) {
      setFeedback({ type: "error", message: error instanceof Error ? error.message : "Failed to load review queue." });
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }

  useEffect(() => {
    if (isLoading) return;
    if (!user) {
      router.replace("/login?redirect=/admin/activities/reviews");
      return;
    }
    if (!isCoordinator) {
      router.replace("/member");
      return;
    }
    void loadQueue(true);
    // Reload on authentication/role change; refresh button handles subsequent updates.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [user, isCoordinator, isLoading, router]);

  function beginReview(item: ReviewItem) {
    setActiveId(item.id);
    setDecision("approved");
    setReason("");
    setFeedback(null);
  }

  async function submitReview(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!activeId || submitting) return;
    if (reason.trim().length < 5) {
      setFeedback({ type: "error", message: "Provide a clear reason of at least five characters." });
      return;
    }
    setSubmitting(true);
    setFeedback(null);
    try {
      await apiRequest(`/participations/claims/${encodeURIComponent(activeId)}/review`, {
        method: "POST",
        body: JSON.stringify({ decision, reason: reason.trim() }),
      });
      setActiveId(null);
      setReason("");
      setFeedback({ type: "success", message: `Participation claim ${decision} successfully.` });
      await loadQueue();
    } catch (error: unknown) {
      setFeedback({ type: "error", message: error instanceof Error ? error.message : "Review submission failed." });
    } finally {
      setSubmitting(false);
    }
  }

  if (isLoading || (user && isCoordinator && loading)) return <Spinner text="Loading coordinator review queue..." />;
  if (!user || !isCoordinator) return null;

  return (
    <main className="max-w-5xl mx-auto px-4 sm:px-6 py-10 space-y-6">
      <Link href="/admin/activities" className="inline-flex items-center gap-2 text-sm text-gray-600 hover:text-emerald-800">
        <ArrowLeft className="w-4 h-4" /> Back to activity management
      </Link>
      <header className="rounded-2xl bg-gradient-to-r from-emerald-950 to-emerald-800 text-white p-6 sm:p-8">
        <div className="flex items-center gap-2 text-xs uppercase tracking-wide font-semibold text-emerald-200">
          <ShieldCheck className="w-4 h-4" /> Coordinator workspace
        </div>
        <h1 className="text-2xl font-extrabold mt-2">Participation Review Queue</h1>
        <p className="text-sm text-emerald-100 mt-2">Review participation evidence and appeals in your authorized jurisdiction.</p>
      </header>
      {feedback && <Alert type={feedback.type}>{feedback.message}</Alert>}
      <section className="bg-white border border-gray-200 rounded-2xl shadow-sm p-5 sm:p-6 space-y-5">
        <div className="flex flex-wrap justify-between items-center gap-3">
          <h2 className="flex items-center gap-2 font-bold text-gray-900"><ClipboardList className="w-5 h-5 text-emerald-800" /> Claims awaiting review ({items.length})</h2>
          <button type="button" onClick={() => void loadQueue()} disabled={refreshing || submitting}
            className="inline-flex items-center gap-2 border border-gray-300 rounded-lg px-3 py-2 text-xs font-semibold hover:bg-gray-50 disabled:opacity-50">
            <RefreshCw className="w-4 h-4" /> {refreshing ? "Refreshing..." : "Refresh queue"}
          </button>
        </div>
        {items.length === 0 ? (
          <p className="rounded-xl bg-gray-50 border border-gray-100 p-6 text-sm text-gray-600 text-center">No claims currently awaiting review.</p>
        ) : (
          <div className="space-y-4">
            {items.map(item => (
              <article key={item.id} className="border border-gray-200 rounded-xl p-4 sm:p-5 space-y-3">
                <div className="flex flex-wrap justify-between items-start gap-2">
                  <div>
                    <h3 className="font-bold text-gray-900">{item.event_title || "Cleanup participation"}</h3>
                    <p className="text-xs text-gray-500 mt-1">Member: {item.member_name || item.display_name || item.user_id || "Not provided"}</p>
                    {item.submitted_at && <p className="text-xs text-gray-500 mt-1">Submitted: {new Date(item.submitted_at).toLocaleString("en-NG", { timeZone: "Africa/Lagos" })}</p>}
                  </div>
                  <span className="rounded-full bg-amber-50 text-amber-900 border border-amber-200 px-3 py-1 text-xs font-semibold capitalize">{item.status || "Pending"}</span>
                </div>
                {item.description && <p className="text-sm text-gray-700 whitespace-pre-wrap"><strong>Contribution:</strong> {item.description}</p>}
                {item.evidence_notes && <p className="text-sm text-gray-600 whitespace-pre-wrap"><strong>Evidence:</strong> {item.evidence_notes}</p>}
                {item.appeal_reason && <p className="text-sm text-amber-900 whitespace-pre-wrap"><strong>Appeal:</strong> {item.appeal_reason}</p>}
                {item.appeals?.map((appeal, index) => (appeal.reason || appeal.appeal_reason) && (
                  <p key={index} className="text-sm text-amber-900 bg-amber-50 rounded-lg p-3 whitespace-pre-wrap"><strong>Appeal:</strong> {appeal.reason || appeal.appeal_reason}</p>
                ))}
                {activeId === item.id ? (
                  <form onSubmit={submitReview} className="border-t border-gray-100 pt-4 space-y-3">
                    <fieldset className="flex flex-wrap gap-4 text-sm text-gray-800">
                      <legend className="font-semibold mb-2">Decision</legend>
                      <label className="flex items-center gap-2"><input type="radio" checked={decision === "approved"} onChange={() => setDecision("approved")} /> Approve</label>
                      <label className="flex items-center gap-2"><input type="radio" checked={decision === "rejected"} onChange={() => setDecision("rejected")} /> Reject</label>
                    </fieldset>
                    <div>
                      <label htmlFor={`reason-${item.id}`} className="block text-sm font-semibold text-gray-800 mb-2">Review reason</label>
                      <textarea id={`reason-${item.id}`} required minLength={5} maxLength={2000} rows={3}
                        value={reason} onChange={event => setReason(event.target.value)}
                        placeholder="Explain the evidence and basis for your decision"
                        className="w-full border border-gray-300 rounded-xl px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-emerald-600" />
                    </div>
                    <div className="flex flex-wrap gap-2">
                      <button type="submit" disabled={submitting || reason.trim().length < 5}
                        className="px-4 py-2 rounded-lg bg-emerald-800 hover:bg-emerald-900 text-white text-sm font-semibold disabled:opacity-50">
                        {submitting ? "Submitting..." : `Confirm ${decision}`}
                      </button>
                      <button type="button" disabled={submitting} onClick={() => setActiveId(null)} className="px-4 py-2 rounded-lg border border-gray-300 text-sm font-semibold">Cancel</button>
                    </div>
                  </form>
                ) : (
                  <button type="button" onClick={() => beginReview(item)} disabled={submitting}
                    className="px-4 py-2 bg-emerald-50 hover:bg-emerald-100 text-emerald-900 border border-emerald-200 rounded-lg text-sm font-semibold">Review claim</button>
                )}
              </article>
            ))}
          </div>
        )}
      </section>
      <p className="text-xs text-gray-500">Only backend-authorized coordinators may review claims. The backend enforces jurisdiction and prevents self-approval. Completed claims may disappear from the pending queue.</p>
    </main>
  );
}

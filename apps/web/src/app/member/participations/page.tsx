"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  FileText,
  Clock,
  CheckCircle2,
  XCircle,
  AlertTriangle,
  ArrowLeft,
  Send,
  PlusCircle,
} from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import { ParticipationClaim, EventInstance, EventRegistration } from "@/lib/types";
import { Badge } from "@/components/ui/Badge";
import { Alert } from "@/components/ui/Alert";
import { Spinner } from "@/components/ui/Spinner";
import { EmptyState } from "@/components/ui/EmptyState";

export default function MemberParticipationsPage() {
  const router = useRouter();
  const { user, isLoading } = useAuth();

  const [claims, setClaims] = useState<ParticipationClaim[]>([]);
  const [events, setEvents] = useState<EventInstance[]>([]);
  const [loading, setLoading] = useState(true);

  // Submission Form State
  const [showSubmitModal, setShowSubmitModal] = useState(false);
  const [selectedEventId, setSelectedEventId] = useState("");
  const [description, setDescription] = useState("");
  const [evidenceNotes, setEvidenceNotes] = useState("");
  const [submitting, setSubmitting] = useState(false);

  // Appeal Modal State
  const [appealClaimId, setAppealClaimId] = useState<string | null>(null);
  const [appealReason, setAppealReason] = useState("");
  const [appealing, setAppealing] = useState(false);

  const [feedback, setFeedback] = useState<{ type: "success" | "error"; message: string } | null>(null);

  const loadData = async () => {
    try {
      const [claimsData, eventsData] = await Promise.all([
        apiRequest<ParticipationClaim[]>("/participations/my"),
        apiRequest<EventInstance[]>("/activities/events?status=all"),
      ]);
      setClaims(claimsData || []);
      setEvents(eventsData || []);
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to load participation data" });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!isLoading && !user) {
      router.push("/login?redirect=/member/participations");
      return;
    }
    if (user) {
      loadData();
    }
  }, [user, isLoading, router]);

  const handleSubmitEvidence = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setFeedback(null);

    try {
      await apiRequest("/participations", {
        method: "POST",
        body: JSON.stringify({
          event_id: selectedEventId,
          description,
          evidence_notes: evidenceNotes || undefined,
        }),
      });

      setFeedback({ type: "success", message: "Participation evidence submitted! It has entered the coordinator review queue." });
      setShowSubmitModal(false);
      setSelectedEventId("");
      setDescription("");
      setEvidenceNotes("");
      await loadData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Submission failed" });
    } finally {
      setSubmitting(false);
    }
  };

  const handleAppeal = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!appealClaimId) return;

    setAppealing(true);
    setFeedback(null);

    try {
      await apiRequest(`/participations/${appealClaimId}/appeal`, {
        method: "POST",
        body: JSON.stringify({ appeal_reason: appealReason }),
      });

      setFeedback({ type: "success", message: "Appeal submitted successfully. A coordinator will review the decision." });
      setAppealClaimId(null);
      setAppealReason("");
      await loadData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Appeal submission failed" });
    } finally {
      setAppealing(false);
    }
  };

  if (isLoading || loading) return <Spinner text="Loading participation history..." />;
  if (!user) return null;

  return (
    <div className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 py-12 space-y-8">
      <Link
        href="/member"
        className="inline-flex items-center text-xs font-semibold text-gray-500 hover:text-gray-900"
      >
        <ArrowLeft className="w-4 h-4 mr-1" />
        Back to Dashboard
      </Link>

      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900 tracking-tight">
            Participation & Evidence History
          </h1>
          <p className="text-xs text-gray-600 mt-1">
            Track submitted attendance reports, coordinator decisions, badges earned, and appeals.
          </p>
        </div>

        <button
          type="button"
          onClick={() => setShowSubmitModal(true)}
          className="inline-flex items-center gap-1.5 px-4 py-2.5 rounded-xl text-sm font-semibold text-white bg-emerald-800 hover:bg-emerald-900 shadow-sm transition"
        >
          <PlusCircle className="w-4 h-4" />
          Submit Activity Evidence
        </button>
      </div>

      {feedback && <Alert type={feedback.type}>{feedback.message}</Alert>}

      {/* Submission Modal Form */}
      {showSubmitModal && (
        <div className="p-6 bg-white rounded-2xl border-2 border-emerald-300 shadow-lg space-y-5">
          <div className="flex justify-between items-center">
            <h2 className="text-base font-bold text-gray-900">Submit Participation Evidence</h2>
            <button
              onClick={() => setShowSubmitModal(false)}
              className="text-gray-400 hover:text-gray-600 text-sm font-medium"
            >
              Cancel
            </button>
          </div>

          <form onSubmit={handleSubmitEvidence} className="space-y-4">
            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">
                Select Attended Activity Venue *
              </label>
              <select
                required
                value={selectedEventId}
                onChange={(e) => setSelectedEventId(e.target.value)}
                className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:ring-emerald-700 focus:border-emerald-700 bg-white"
              >
                <option value="">Choose an activity...</option>
                {events.map((ev) => (
                  <option key={ev.id} value={ev.id}>
                    {ev.title} ({ev.state_name} • {new Date(ev.start_time).toLocaleDateString()})
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">
                Description of Contribution * (Minimum 10 characters)
              </label>
              <textarea
                required
                minLength={10}
                rows={3}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Describe your role: e.g. Cleared storm drain, helped load waste disposal truck, or distributed safety gloves..."
                className="w-full border border-gray-300 rounded-lg p-3 text-sm focus:ring-emerald-700 focus:border-emerald-700"
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">
                Supporting Evidence Notes (Optional)
              </label>
              <input
                type="text"
                value={evidenceNotes}
                onChange={(e) => setEvidenceNotes(e.target.value)}
                placeholder="Team leader name, specific section assigned, or photo reference..."
                className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:ring-emerald-700 focus:border-emerald-700"
              />
            </div>

            <div className="flex justify-end gap-3 pt-2">
              <button
                type="button"
                onClick={() => setShowSubmitModal(false)}
                className="px-4 py-2 border border-gray-300 rounded-xl text-xs font-semibold text-gray-700 hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={submitting}
                className="px-5 py-2 bg-emerald-800 hover:bg-emerald-900 text-white rounded-xl text-xs font-semibold transition"
              >
                {submitting ? "Submitting..." : "Submit for Verification"}
              </button>
            </div>
          </form>
        </div>
      )}

      {/* Appeal Form Modal */}
      {appealClaimId && (
        <div className="p-6 bg-white rounded-2xl border-2 border-amber-300 shadow-lg space-y-4">
          <div className="flex justify-between items-center">
            <h2 className="text-base font-bold text-gray-900">Submit Participation Appeal</h2>
            <button
              onClick={() => setAppealClaimId(null)}
              className="text-gray-400 hover:text-gray-600 text-sm font-medium"
            >
              Cancel
            </button>
          </div>

          <p className="text-xs text-gray-600">
            If your attendance evidence was rejected in error, explain why your participation was valid. An authorized coordinator will review your appeal.
          </p>

          <form onSubmit={handleAppeal} className="space-y-4">
            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">
                Reason for Appeal * (Minimum 10 characters)
              </label>
              <textarea
                required
                minLength={10}
                rows={3}
                value={appealReason}
                onChange={(e) => setAppealReason(e.target.value)}
                placeholder="State why the rejection was mistaken (e.g. signed physical roster on-site, worked with Team Lead, etc.)..."
                className="w-full border border-gray-300 rounded-lg p-3 text-sm focus:ring-emerald-700 focus:border-emerald-700"
              />
            </div>

            <div className="flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setAppealClaimId(null)}
                className="px-4 py-2 border border-gray-300 rounded-xl text-xs font-semibold text-gray-700 hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={appealing}
                className="px-5 py-2 bg-amber-700 hover:bg-amber-800 text-white rounded-xl text-xs font-semibold transition"
              >
                {appealing ? "Submitting Appeal..." : "Submit Appeal"}
              </button>
            </div>
          </form>
        </div>
      )}

      {/* Claims List */}
      {claims.length === 0 ? (
        <EmptyState
          title="No Participation Records Yet"
          description="You have not submitted participation evidence for any activity. Once you attend a cleanup, submit your report to earn your First Participation badge!"
          actionText="Submit Evidence"
          onAction={() => setShowSubmitModal(true)}
        />
      ) : (
        <div className="space-y-4">
          {claims.map((claim) => (
            <div
              key={claim.id}
              className="bg-white rounded-2xl border border-gray-200 p-6 shadow-sm space-y-4 text-xs"
            >
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 border-b border-gray-100 pb-3">
                <div>
                  <h3 className="font-bold text-gray-900 text-base">
                    {claim.event_title || "Sanitation Exercise"}
                  </h3>
                  <div className="text-gray-500 text-[11px] mt-0.5">
                    Submitted: {new Date(claim.submitted_at).toLocaleDateString("en-NG", { year: "numeric", month: "short", day: "numeric" })}
                  </div>
                </div>

                <Badge
                  variant={
                    claim.status === "verified"
                      ? "success"
                      : claim.status === "rejected"
                      ? "danger"
                      : claim.status === "appealed"
                      ? "warning"
                      : "info"
                  }
                  size="md"
                >
                  {claim.status.toUpperCase()}
                </Badge>
              </div>

              <div className="space-y-2">
                <div>
                  <span className="font-semibold text-gray-700">Submitted Description:</span>
                  <p className="text-gray-800 mt-0.5 leading-relaxed bg-gray-50 p-3 rounded-lg border border-gray-100">
                    {claim.description}
                  </p>
                </div>

                {claim.evidence_notes && (
                  <div className="text-gray-600">
                    <strong>Evidence Notes:</strong> {claim.evidence_notes}
                  </div>
                )}
              </div>

              {/* Review Decision Information */}
              {claim.review_decision && (
                <div
                  className={`p-3.5 rounded-xl border text-xs ${
                    claim.review_decision === "approved" || claim.review_decision === "approved_on_appeal"
                      ? "bg-emerald-50 border-emerald-200 text-emerald-950"
                      : "bg-rose-50 border-rose-200 text-rose-950"
                  }`}
                >
                  <div className="font-semibold mb-0.5">
                    Coordinator Review Decision: {claim.review_decision.replace("_", " ").toUpperCase()}
                  </div>
                  <div>
                    <strong>Reviewer Reason:</strong> {claim.review_reason}
                  </div>
                  {claim.reviewed_at && (
                    <div className="text-[11px] opacity-75 mt-1">
                      Reviewed on: {new Date(claim.reviewed_at).toLocaleDateString()}
                    </div>
                  )}
                </div>
              )}

              {/* Action buttons (Appeal if rejected) */}
              {claim.status === "rejected" && claim.appeals.length === 0 && (
                <div className="pt-2 flex justify-end">
                  <button
                    type="button"
                    onClick={() => setAppealClaimId(claim.id)}
                    className="px-4 py-2 text-xs font-semibold text-amber-800 bg-amber-50 hover:bg-amber-100 border border-amber-300 rounded-lg transition"
                  >
                    Appeal Rejection Decision
                  </button>
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

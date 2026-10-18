"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  FileCheck,
  CheckCircle,
  XCircle,
  AlertCircle,
  ArrowLeft,
  Clock,
  User,
  ShieldAlert,
} from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import { ParticipationClaim } from "@/lib/types";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";
import { EmptyState } from "@/components/ui/EmptyState";

export default function AdminReviewsPage() {
  const router = useRouter();
  const { user, isCoordinator, isLoading } = useAuth();

  const [claims, setClaims] = useState<ParticipationClaim[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<"pending" | "appealed" | "all">("pending");

  // Review Modal
  const [reviewClaim, setReviewClaim] = useState<ParticipationClaim | null>(null);
  const [decision, setDecision] = useState<"approved" | "rejected">("approved");
  const [reason, setReason] = useState("");
  const [reviewing, setReviewing] = useState(false);

  // Appeal Review Modal
  const [appealClaim, setAppealClaim] = useState<ParticipationClaim | null>(null);
  const [appealDecision, setAppealDecision] = useState<"accepted" | "rejected">("accepted");
  const [appealReason, setAppealReason] = useState("");
  const [reviewingAppeal, setReviewingAppeal] = useState(false);

  const [feedback, setFeedback] = useState<{ type: "success" | "error"; message: string } | null>(null);

  const loadClaims = async () => {
    try {
      const data = await apiRequest<ParticipationClaim[]>("/participations/review-queue?status=all");
      setClaims(data || []);
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to load review queue" });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!isLoading && (!user || !isCoordinator)) {
      router.push("/login?redirect=/admin/reviews");
      return;
    }
    if (user && isCoordinator) {
      loadClaims();
    }
  }, [user, isCoordinator, isLoading, router]);

  const handleReviewSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!reviewClaim) return;

    if (reviewClaim.user_id === user?.id) {
      setFeedback({
        type: "error",
        message: "Self-approval prohibited: coordinators cannot review their own participation claims.",
      });
      return;
    }

    setReviewing(true);
    setFeedback(null);

    try {
      await apiRequest(`/participations/${reviewClaim.id}/review`, {
        method: "POST",
        body: JSON.stringify({ decision, reason }),
      });

      setFeedback({
        type: "success",
        message: `Participation claim ${decision === "approved" ? "approved" : "rejected"} successfully.`,
      });
      setReviewClaim(null);
      setReason("");
      await loadClaims();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Review submission failed" });
    } finally {
      setReviewing(false);
    }
  };

  const handleAppealReviewSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!appealClaim || appealClaim.appeals.length === 0) return;

    const appealId = appealClaim.appeals[0].id;
    setReviewingAppeal(true);
    setFeedback(null);

    try {
      await apiRequest(`/participations/appeals/${appealId}/review`, {
        method: "POST",
        body: JSON.stringify({ decision: appealDecision, reason: appealReason }),
      });

      setFeedback({
        type: "success",
        message: `Appeal decision recorded as ${appealDecision}.`,
      });
      setAppealClaim(null);
      setAppealReason("");
      await loadClaims();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Appeal review failed" });
    } finally {
      setReviewingAppeal(false);
    }
  };

  if (isLoading || loading) return <Spinner text="Loading review queue..." />;

  const filteredClaims = claims.filter((c) => {
    if (activeTab === "pending") return c.status === "pending";
    if (activeTab === "appealed") return c.status === "appealed";
    return true;
  });

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-8">
      <Link
        href="/admin"
        className="inline-flex items-center text-xs font-semibold text-gray-500 hover:text-gray-900"
      >
        <ArrowLeft className="w-4 h-4 mr-1" />
        Back to Admin Workspace
      </Link>

      <div className="space-y-2">
        <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900 tracking-tight">
          Participation Verification Queue
        </h1>
        <p className="text-xs text-gray-600">
          Verify attendance evidence submitted by volunteers. Approving a member&apos;s first activity automatically grants the &ldquo;First Participation&rdquo; badge.
        </p>
      </div>

      <Alert type="info">
        <strong>Dual Approval Safeguard:</strong> The system enforces server-side access controls preventing coordinators from approving their own participation claims.
      </Alert>

      {feedback && <Alert type={feedback.type}>{feedback.message}</Alert>}

      {/* Tabs */}
      <div className="flex border-b border-gray-200 gap-6 text-xs font-semibold">
        <button
          onClick={() => setActiveTab("pending")}
          className={`pb-3 border-b-2 transition ${
            activeTab === "pending"
              ? "border-emerald-800 text-emerald-950"
              : "border-transparent text-gray-500 hover:text-gray-700"
          }`}
        >
          Pending Reviews ({claims.filter((c) => c.status === "pending").length})
        </button>

        <button
          onClick={() => setActiveTab("appealed")}
          className={`pb-3 border-b-2 transition ${
            activeTab === "appealed"
              ? "border-amber-600 text-amber-900"
              : "border-transparent text-gray-500 hover:text-gray-700"
          }`}
        >
          Appeals Queue ({claims.filter((c) => c.status === "appealed").length})
        </button>

        <button
          onClick={() => setActiveTab("all")}
          className={`pb-3 border-b-2 transition ${
            activeTab === "all"
              ? "border-emerald-800 text-emerald-950"
              : "border-transparent text-gray-500 hover:text-gray-700"
          }`}
        >
          All Claims ({claims.length})
        </button>
      </div>

      {/* Review Modal Form */}
      {reviewClaim && (
        <div className="p-6 bg-white rounded-2xl border-2 border-emerald-400 shadow-xl space-y-5">
          <div className="flex justify-between items-center">
            <h2 className="text-base font-bold text-gray-900">
              Review Participation Claim — {reviewClaim.user_display_name}
            </h2>
            <button
              onClick={() => setReviewClaim(null)}
              className="text-gray-400 hover:text-gray-600 text-sm font-medium"
            >
              Cancel
            </button>
          </div>

          <div className="p-4 bg-gray-50 rounded-xl text-xs space-y-2 border border-gray-100">
            <div>
              <strong>Event:</strong> {reviewClaim.event_title}
            </div>
            <div>
              <strong>Member Reference:</strong> {reviewClaim.user_membership_ref}
            </div>
            <div>
              <strong>Submitted Evidence:</strong>
              <p className="mt-1 text-gray-800 p-2.5 bg-white rounded border border-gray-200">
                {reviewClaim.description}
              </p>
            </div>
            {reviewClaim.evidence_notes && (
              <div>
                <strong>Additional Notes:</strong> {reviewClaim.evidence_notes}
              </div>
            )}
          </div>

          {reviewClaim.user_id === user?.id && (
            <Alert type="error" title="Self-Approval Prohibited">
              You submitted this participation claim. Another coordinator or administrator must review it.
            </Alert>
          )}

          <form onSubmit={handleReviewSubmit} className="space-y-4">
            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">Decision *</label>
              <div className="flex gap-4">
                <label className="flex items-center gap-2 cursor-pointer text-xs font-medium">
                  <input
                    type="radio"
                    name="decision"
                    value="approved"
                    checked={decision === "approved"}
                    onChange={() => setDecision("approved")}
                    className="text-emerald-800 focus:ring-emerald-700"
                  />
                  <span>Approve Attendance (Awards Badge if first)</span>
                </label>

                <label className="flex items-center gap-2 cursor-pointer text-xs font-medium text-rose-700">
                  <input
                    type="radio"
                    name="decision"
                    value="rejected"
                    checked={decision === "rejected"}
                    onChange={() => setDecision("rejected")}
                    className="text-rose-600 focus:ring-rose-500"
                  />
                  <span>Reject Claim</span>
                </label>
              </div>
            </div>

            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">
                Reason / Field Assessment Notes * (Minimum 3 characters)
              </label>
              <input
                type="text"
                required
                minLength={3}
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                placeholder="e.g. Confirmed on physical site roster and completed section clearing"
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
              />
            </div>

            <div className="flex justify-end gap-3 pt-2">
              <button
                type="button"
                onClick={() => setReviewClaim(null)}
                className="px-4 py-2 border border-gray-300 rounded-xl text-xs font-semibold text-gray-700 hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={reviewing || reviewClaim.user_id === user?.id}
                className="px-5 py-2 bg-emerald-800 hover:bg-emerald-900 text-white rounded-xl text-xs font-semibold transition disabled:opacity-50"
              >
                {reviewing ? "Saving Decision..." : "Submit Review Decision"}
              </button>
            </div>
          </form>
        </div>
      )}

      {/* Appeal Review Modal */}
      {appealClaim && (
        <div className="p-6 bg-white rounded-2xl border-2 border-amber-400 shadow-xl space-y-5">
          <div className="flex justify-between items-center">
            <h2 className="text-base font-bold text-gray-900">
              Review Appeal — {appealClaim.user_display_name}
            </h2>
            <button
              onClick={() => setAppealClaim(null)}
              className="text-gray-400 hover:text-gray-600 text-sm font-medium"
            >
              Cancel
            </button>
          </div>

          <div className="p-4 bg-amber-50/60 rounded-xl text-xs space-y-2 border border-amber-200">
            <div>
              <strong>Original Rejection Reason:</strong> {appealClaim.review_reason}
            </div>
            <div>
              <strong>Member&apos;s Stated Appeal:</strong>
              <p className="mt-1 text-gray-900 p-2.5 bg-white rounded border border-amber-200">
                {appealClaim.appeals[0]?.appeal_reason}
              </p>
            </div>
          </div>

          <form onSubmit={handleAppealReviewSubmit} className="space-y-4">
            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">Appeal Decision *</label>
              <div className="flex gap-4">
                <label className="flex items-center gap-2 cursor-pointer text-xs font-medium text-emerald-800">
                  <input
                    type="radio"
                    name="appealDecision"
                    value="accepted"
                    checked={appealDecision === "accepted"}
                    onChange={() => setAppealDecision("accepted")}
                    className="text-emerald-800 focus:ring-emerald-700"
                  />
                  <span>Accept Appeal (Verify Attendance)</span>
                </label>

                <label className="flex items-center gap-2 cursor-pointer text-xs font-medium text-rose-700">
                  <input
                    type="radio"
                    name="appealDecision"
                    value="rejected"
                    checked={appealDecision === "rejected"}
                    onChange={() => setAppealDecision("rejected")}
                    className="text-rose-600 focus:ring-rose-500"
                  />
                  <span>Uphold Rejection</span>
                </label>
              </div>
            </div>

            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">
                Decision Notes * (Minimum 3 characters)
              </label>
              <input
                type="text"
                required
                minLength={3}
                value={appealReason}
                onChange={(e) => setAppealReason(e.target.value)}
                placeholder="State basis for accepting or upholding appeal..."
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
              />
            </div>

            <div className="flex justify-end gap-3 pt-2">
              <button
                type="button"
                onClick={() => setAppealClaim(null)}
                className="px-4 py-2 border border-gray-300 rounded-xl text-xs font-semibold text-gray-700 hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={reviewingAppeal}
                className="px-5 py-2 bg-emerald-800 hover:bg-emerald-900 text-white rounded-xl text-xs font-semibold transition"
              >
                {reviewingAppeal ? "Saving..." : "Confirm Appeal Decision"}
              </button>
            </div>
          </form>
        </div>
      )}

      {/* Claims Table */}
      {filteredClaims.length === 0 ? (
        <EmptyState
          title="No Claims in Queue"
          description={`There are currently no participation claims with status '${activeTab}'.`}
        />
      ) : (
        <div className="bg-white rounded-2xl border border-gray-200 overflow-hidden shadow-sm">
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-gray-200 text-xs">
              <thead className="bg-gray-50 text-gray-600 font-semibold uppercase text-[11px]">
                <tr>
                  <th className="px-4 py-3 text-left">Member</th>
                  <th className="px-4 py-3 text-left">Activity</th>
                  <th className="px-4 py-3 text-left">Evidence Description</th>
                  <th className="px-4 py-3 text-left">Submitted Date</th>
                  <th className="px-4 py-3 text-left">Status</th>
                  <th className="px-4 py-3 text-right">Review Action</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {filteredClaims.map((claim) => (
                  <tr key={claim.id} className="hover:bg-gray-50/60 transition">
                    <td className="px-4 py-3.5 whitespace-nowrap">
                      <div className="font-bold text-gray-900">{claim.user_display_name}</div>
                      <div className="text-[11px] font-mono text-gray-500">{claim.user_membership_ref}</div>
                    </td>
                    <td className="px-4 py-3.5 whitespace-nowrap">
                      <div className="font-semibold text-gray-800">{claim.event_title}</div>
                      <div className="text-[11px] text-gray-500">
                        {claim.event_date ? new Date(claim.event_date).toLocaleDateString() : ""}
                      </div>
                    </td>
                    <td className="px-4 py-3.5 max-w-xs">
                      <p className="line-clamp-2 text-gray-700">{claim.description}</p>
                    </td>
                    <td className="px-4 py-3.5 whitespace-nowrap text-gray-500">
                      {new Date(claim.submitted_at).toLocaleDateString()}
                    </td>
                    <td className="px-4 py-3.5 whitespace-nowrap">
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
                        size="sm"
                      >
                        {claim.status.toUpperCase()}
                      </Badge>
                    </td>
                    <td className="px-4 py-3.5 whitespace-nowrap text-right">
                      {claim.status === "pending" && (
                        <button
                          type="button"
                          onClick={() => setReviewClaim(claim)}
                          className="px-3 py-1.5 bg-emerald-800 hover:bg-emerald-900 text-white font-semibold rounded-lg text-[11px] transition"
                        >
                          Review Claim
                        </button>
                      )}

                      {claim.status === "appealed" && (
                        <button
                          type="button"
                          onClick={() => setAppealClaim(claim)}
                          className="px-3 py-1.5 bg-amber-700 hover:bg-amber-800 text-white font-semibold rounded-lg text-[11px] transition"
                        >
                          Review Appeal
                        </button>
                      )}

                      {(claim.status === "verified" || claim.status === "rejected") && (
                        <span className="text-[11px] text-gray-400 italic">Completed</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}

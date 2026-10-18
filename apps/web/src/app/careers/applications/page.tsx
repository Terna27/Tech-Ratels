"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  FileText,
  Calendar,
  Building2,
  Clock,
  CheckCircle2,
  AlertCircle,
  XCircle,
  ArrowRight,
  ExternalLink,
  ChevronDown,
  ChevronUp,
  History,
  Video,
  MapPin,
  Phone,
  Sparkles,
} from "lucide-react";
import { apiRequest } from "@/lib/api";
import { CareersApplication, CareersInterview, CareersApplicationHistory } from "@/lib/types";
import { useAuth } from "@/lib/auth-context";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";
import { Badge } from "@/components/ui/Badge";
import { EmptyState } from "@/components/ui/EmptyState";

export default function ApplicationsTrackerPage() {
  const router = useRouter();
  const { user, isLoading: authLoading } = useAuth();

  const [applications, setApplications] = useState<CareersApplication[]>([]);
  const [interviews, setInterviews] = useState<CareersInterview[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Withdrawal Modal State
  const [withdrawApp, setWithdrawApp] = useState<CareersApplication | null>(null);
  const [withdrawalReason, setWithdrawalReason] = useState("");
  const [withdrawing, setWithdrawing] = useState(false);
  const [withdrawError, setWithdrawError] = useState<string | null>(null);

  // History Modal State
  const [selectedAppHistory, setSelectedAppHistory] = useState<{
    app: CareersApplication;
    history: CareersApplicationHistory[];
  } | null>(null);
  const [historyLoading, setHistoryLoading] = useState(false);

  useEffect(() => {
    if (!authLoading && !user) {
      router.push("/login?redirect=/careers/applications");
      return;
    }

    if (user) {
      loadData();
    }
  }, [user, authLoading]);

  const loadData = async () => {
    setLoading(true);
    setError(null);
    try {
      const [appsRes, intsRes] = await Promise.all([
        apiRequest<CareersApplication[]>("/careers/applications"),
        apiRequest<CareersInterview[]>("/careers/interviews"),
      ]);

      setApplications(Array.isArray(appsRes) ? appsRes : []);
      setInterviews(Array.isArray(intsRes) ? intsRes : []);
    } catch (err: any) {
      setError(err.message || "Failed to load application records");
    } finally {
      setLoading(false);
    }
  };

  const handleWithdrawSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!withdrawApp) return;

    setWithdrawing(true);
    setWithdrawError(null);
    try {
      await apiRequest(`/careers/applications/${withdrawApp.id}/withdraw`, {
        method: "POST",
        body: JSON.stringify({ reason: withdrawalReason.trim() }),
      });

      setWithdrawApp(null);
      setWithdrawalReason("");
      loadData();
    } catch (err: any) {
      setWithdrawError(err.message || "Failed to withdraw application");
    } finally {
      setWithdrawing(false);
    }
  };

  const openHistoryModal = async (app: CareersApplication) => {
    setHistoryLoading(true);
    try {
      const hist = await apiRequest<CareersApplicationHistory[]>(
        `/careers/applications/${app.id}/history`
      );
      setSelectedAppHistory({
        app,
        history: Array.isArray(hist) ? hist : [],
      });
    } catch (err: any) {
      setError(err.message || "Failed to load application history");
    } finally {
      setHistoryLoading(false);
    }
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case "submitted":
        return <Badge variant="info">Submitted</Badge>;
      case "under_review":
        return <Badge variant="warning">Under Review</Badge>;
      case "shortlisted":
        return <Badge variant="primary">Shortlisted</Badge>;
      case "interview":
        return <Badge variant="success">Interview</Badge>;
      case "offered":
        return <Badge variant="success">Offer Extended</Badge>;
      case "hired":
        return <Badge variant="success">Hired</Badge>;
      case "rejected":
        return <Badge variant="danger">Not Selected</Badge>;
      case "withdrawn":
        return <Badge variant="neutral">Withdrawn</Badge>;
      default:
        return <Badge variant="neutral">{status}</Badge>;
    }
  };

  if (authLoading || loading) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center py-20">
        <Spinner text="Loading applications and scheduled interviews..." />
      </div>
    );
  }

  const activeInterviews = interviews.filter((i) => i.status === "scheduled");

  return (
    <div className="min-h-screen bg-gray-50 pb-20">
      {/* Header */}
      <section className="bg-emerald-900 text-white py-10 px-4 sm:px-6 lg:px-8 border-b border-emerald-800">
        <div className="max-w-6xl mx-auto flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div>
            <h1 className="text-2xl sm:text-3xl font-extrabold tracking-tight">
              My Applications & Interviews
            </h1>
            <p className="text-emerald-100 text-sm mt-1">
              Track recruitment decisions, upcoming interview appointments, and status milestones.
            </p>
          </div>
          <div className="flex items-center gap-3">
            <Link
              href="/careers"
              className="px-4 py-2 rounded-lg bg-emerald-700 hover:bg-emerald-600 text-xs font-medium text-white transition shadow-sm border border-emerald-600"
            >
              Browse Openings
            </Link>
            <Link
              href="/careers/profile"
              className="px-4 py-2 rounded-lg bg-white/10 hover:bg-white/20 text-xs font-medium text-white transition border border-emerald-700"
            >
              My Profile
            </Link>
          </div>
        </div>
      </section>

      <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8 pt-8 space-y-8">
        {error && (
          <Alert type="error" title="Error">
            {error}
          </Alert>
        )}

        {/* Scheduled Interviews Alert Card */}
        {activeInterviews.length > 0 && (
          <div className="bg-white rounded-xl border border-emerald-200 p-6 shadow-sm space-y-4">
            <div className="flex items-center gap-2 text-emerald-900 font-bold text-base">
              <Calendar className="w-5 h-5 text-emerald-700" />
              <span>Upcoming Scheduled Interviews ({activeInterviews.length})</span>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {activeInterviews.map((item) => (
                <div
                  key={item.id}
                  className="p-4 rounded-xl border border-emerald-100 bg-emerald-50/50 space-y-2.5"
                >
                  <div className="flex items-start justify-between gap-2">
                    <div>
                      <h4 className="font-bold text-gray-900 text-sm">{item.job_title}</h4>
                      <p className="text-xs text-gray-600">{item.employer_name}</p>
                    </div>
                    <Badge variant="success" size="sm">
                      {item.format === "video"
                        ? "Video Call"
                        : item.format === "in_person"
                        ? "In-person"
                        : "Phone Call"}
                    </Badge>
                  </div>

                  <div className="text-xs text-gray-700 flex items-center gap-1.5 pt-1">
                    <Clock className="w-3.5 h-3.5 text-emerald-700" />
                    <strong>
                      {new Date(item.scheduled_time).toLocaleString("en-NG", {
                        weekday: "short",
                        day: "numeric",
                        month: "short",
                        year: "numeric",
                        hour: "2-digit",
                        minute: "2-digit",
                      })}
                    </strong>
                  </div>

                  {item.location_or_link && (
                    <div className="text-xs text-emerald-900 bg-white p-2 rounded border border-emerald-200 break-all">
                      <span className="font-semibold block text-[11px] text-gray-500 uppercase">
                        Meeting Link / Venue:
                      </span>
                      {item.location_or_link.startsWith("http") ? (
                        <a
                          href={item.location_or_link}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="underline hover:text-emerald-700 flex items-center gap-1"
                        >
                          Join Meeting
                          <ExternalLink className="w-3 h-3" />
                        </a>
                      ) : (
                        <span>{item.location_or_link}</span>
                      )}
                    </div>
                  )}

                  {item.instructions && (
                    <p className="text-xs text-gray-600 bg-white p-2 rounded border border-gray-100">
                      <span className="font-semibold text-gray-800">Instructions: </span>
                      {item.instructions}
                    </p>
                  )}
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Applications List */}
        <div className="bg-white rounded-xl border border-gray-200 p-6 shadow-sm">
          <div className="flex items-center justify-between pb-4 border-b border-gray-100 mb-6">
            <h2 className="text-lg font-bold text-gray-900 flex items-center gap-2">
              <FileText className="w-5 h-5 text-emerald-700" />
              Submitted Applications
            </h2>
            <span className="text-xs text-gray-500 font-medium">
              Total: {applications.length}
            </span>
          </div>

          {applications.length === 0 ? (
            <EmptyState
              icon={<FileText className="w-8 h-8 text-gray-400" />}
              title="No Applications Submitted"
              description="You have not submitted any applications yet. Explore active opportunities across Nigeria."
              actionText="Explore Jobs"
              onAction={() => router.push("/careers")}
            />
          ) : (
            <div className="space-y-4">
              {applications.map((app) => {
                const canWithdraw =
                  app.status === "submitted" ||
                  app.status === "under_review" ||
                  app.status === "shortlisted" ||
                  app.status === "interview";

                return (
                  <div
                    key={app.id}
                    className="p-5 rounded-xl border border-gray-200 hover:border-gray-300 transition space-y-4"
                  >
                    <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                      <div>
                        <div className="flex items-center gap-2 mb-1">
                          {getStatusBadge(app.status)}
                          {app.match_score > 0 && (
                            <span className="text-xs font-semibold px-2 py-0.5 rounded-full bg-emerald-50 text-emerald-800 border border-emerald-200 flex items-center gap-1">
                              <Sparkles className="w-3 h-3 text-emerald-600" />
                              {app.match_score}% Match
                            </span>
                          )}
                        </div>
                        <h3 className="text-base font-bold text-gray-900">{app.job_title}</h3>
                        <p className="text-xs text-gray-600 flex items-center gap-2 mt-0.5">
                          <span className="font-medium text-gray-800">{app.employer_name}</span>
                          <span>•</span>
                          <span>
                            Submitted on{" "}
                            {new Date(app.created_at).toLocaleDateString("en-NG", {
                              day: "numeric",
                              month: "short",
                              year: "numeric",
                            })}
                          </span>
                        </p>
                      </div>

                      <div className="flex items-center gap-2">
                        <button
                          type="button"
                          onClick={() => openHistoryModal(app)}
                          className="px-3 py-1.5 rounded-lg border border-gray-200 bg-gray-50 hover:bg-gray-100 text-xs font-medium text-gray-700 transition flex items-center gap-1"
                        >
                          <History className="w-3.5 h-3.5 text-gray-500" />
                          Audit History
                        </button>
                        {canWithdraw && (
                          <button
                            type="button"
                            onClick={() => {
                              setWithdrawApp(app);
                              setWithdrawalReason("");
                              setWithdrawError(null);
                            }}
                            className="px-3 py-1.5 rounded-lg border border-rose-200 bg-rose-50 hover:bg-rose-100 text-xs font-medium text-rose-700 transition"
                          >
                            Withdraw
                          </button>
                        )}
                      </div>
                    </div>

                    {/* Explanatory Match Criteria if available */}
                    {app.match_factors?.criteria_explanation && (
                      <div className="text-xs bg-gray-50 p-3 rounded-lg border border-gray-100 text-gray-600 leading-relaxed">
                        <span className="font-semibold text-gray-800 block mb-0.5">
                          Objective Matching Breakdown:
                        </span>
                        {app.match_factors.criteria_explanation}
                      </div>
                    )}

                    {/* Status Rejection or Withdrawal Notes */}
                    {app.status === "withdrawn" && app.withdrawal_reason && (
                      <div className="text-xs text-gray-600 bg-gray-50 p-2.5 rounded border border-gray-200">
                        <strong>Reason for withdrawal:</strong> {app.withdrawal_reason}
                      </div>
                    )}
                    {app.status === "rejected" && app.rejection_reason && (
                      <div className="text-xs text-rose-800 bg-rose-50 p-2.5 rounded border border-rose-200">
                        <strong>Feedback:</strong> {app.rejection_reason}
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </div>

      {/* Withdrawal Confirmation Dialog */}
      {withdrawApp && (
        <div className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 shadow-2xl border border-gray-200">
            <h3 className="text-lg font-bold text-gray-900 mb-1">Withdraw Application</h3>
            <p className="text-xs text-gray-600 mb-4">
              Are you sure you want to withdraw your application for{" "}
              <strong>{withdrawApp.job_title}</strong>? This action will formally close your
              submission.
            </p>

            {withdrawError && (
              <div className="mb-3">
                <Alert type="error">{withdrawError}</Alert>
              </div>
            )}

            <form onSubmit={handleWithdrawSubmit} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-gray-700 uppercase mb-1">
                  Reason for Withdrawal (Optional)
                </label>
                <textarea
                  rows={3}
                  value={withdrawalReason}
                  onChange={(e) => setWithdrawalReason(e.target.value)}
                  placeholder="e.g. Accepted another position, relocation, etc."
                  className="w-full text-xs p-2.5 rounded-lg border border-gray-300 focus:ring-emerald-700 focus:border-emerald-700"
                />
              </div>

              <div className="flex items-center justify-end gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => setWithdrawApp(null)}
                  className="px-4 py-2 text-xs font-medium text-gray-700 hover:text-gray-900 transition"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={withdrawing}
                  className="px-4 py-2 rounded-lg bg-rose-600 hover:bg-rose-700 text-white font-medium text-xs transition disabled:opacity-50"
                >
                  {withdrawing ? "Withdrawing..." : "Confirm Withdrawal"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* History Audit Modal */}
      {selectedAppHistory && (
        <div className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl max-w-lg w-full p-6 shadow-2xl border border-gray-200">
            <div className="flex items-center justify-between pb-3 border-b border-gray-100 mb-4">
              <div>
                <h3 className="text-base font-bold text-gray-900">Application Audit Trail</h3>
                <p className="text-xs text-gray-500">{selectedAppHistory.app.job_title}</p>
              </div>
              <button
                type="button"
                onClick={() => setSelectedAppHistory(null)}
                className="text-gray-400 hover:text-gray-600 text-sm font-semibold p-1"
              >
                ✕
              </button>
            </div>

            <div className="space-y-3 max-h-80 overflow-y-auto pr-1">
              {selectedAppHistory.history.length === 0 ? (
                <p className="text-xs text-gray-500 italic py-2">
                  No status transition records logged.
                </p>
              ) : (
                selectedAppHistory.history.map((h) => (
                  <div key={h.id} className="text-xs p-3 rounded-lg border border-gray-100 bg-gray-50">
                    <div className="flex items-center justify-between font-semibold text-gray-800">
                      <span>
                        {h.from_status ? `${h.from_status} → ` : ""}
                        {h.to_status}
                      </span>
                      <span className="text-[11px] text-gray-500 font-normal">
                        {new Date(h.created_at).toLocaleString("en-NG", {
                          day: "numeric",
                          month: "short",
                          year: "numeric",
                          hour: "2-digit",
                          minute: "2-digit",
                        })}
                      </span>
                    </div>
                    {h.notes && <p className="text-gray-600 mt-1">{h.notes}</p>}
                  </div>
                ))
              )}
            </div>

            <div className="pt-4 border-t border-gray-100 flex justify-end">
              <button
                type="button"
                onClick={() => setSelectedAppHistory(null)}
                className="px-4 py-2 rounded-lg bg-gray-100 hover:bg-gray-200 text-gray-700 text-xs font-medium transition"
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

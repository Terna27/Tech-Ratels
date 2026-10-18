"use client";

import React, { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import {
  FolderKanban,
  MapPin,
  Calendar,
  AlertCircle,
  CheckCircle2,
  Clock,
  ArrowRight,
  PlusCircle,
  RotateCcw,
  MessageSquare,
  ArrowLeft,
  X,
} from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import { CommunityRequest } from "@/lib/types";
import { Badge } from "@/components/ui/Badge";

export default function MemberCommunityRequestsPage() {
  const router = useRouter();
  const { user, isLoading: authLoading } = useAuth();

  const [requests, setRequests] = useState<CommunityRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Resubmit Modal State
  const [resubmittingReq, setResubmittingReq] = useState<CommunityRequest | null>(null);
  const [resubmitDesc, setResubmitDesc] = useState("");
  const [resubmitLocation, setResubmitLocation] = useState("");
  const [resubmitLoading, setResubmitLoading] = useState(false);
  const [resubmitError, setResubmitError] = useState<string | null>(null);

  useEffect(() => {
    if (!authLoading && !user) {
      router.push("/login?redirect=/projects/requests");
    }
  }, [user, authLoading, router]);

  const loadMyRequests = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await apiRequest<{ requests: CommunityRequest[] }>("/community/requests/mine");
      setRequests(res?.requests || []);
    } catch (err: any) {
      console.error("Failed to load requests", err);
      setError("Failed to retrieve your submitted requests.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (user) {
      loadMyRequests();
    }
  }, [user]);

  const handleOpenResubmit = (req: CommunityRequest) => {
    setResubmittingReq(req);
    setResubmitDesc(req.description);
    setResubmitLocation(req.location_details);
    setResubmitError(null);
  };

  const handleResubmitSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!resubmittingReq) return;
    setResubmitLoading(true);
    setResubmitError(null);

    try {
      await apiRequest(`/community/requests/${resubmittingReq.id}/resubmit`, {
        method: "POST",
        body: JSON.stringify({
          description: resubmitDesc.trim(),
          location_details: resubmitLocation.trim(),
          evidence_image_urls: resubmittingReq.evidence_image_urls || [],
        }),
      });

      setResubmittingReq(null);
      await loadMyRequests();
    } catch (err: any) {
      console.error("Failed to resubmit request", err);
      setResubmitError(err?.message || "Failed to resubmit. Please try again.");
    } finally {
      setResubmitLoading(false);
    }
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case "approved":
        return <Badge variant="success">Approved</Badge>;
      case "verified":
        return <Badge variant="info">Verified on Ground</Badge>;
      case "under_review":
        return <Badge variant="warning">Under Review</Badge>;
      case "info_requested":
        return <Badge variant="danger">More Info Requested</Badge>;
      case "rejected":
        return <Badge variant="danger">Declined</Badge>;
      default:
        return <Badge variant="neutral">Submitted</Badge>;
    }
  };

  if (authLoading) {
    return (
      <div className="py-20 text-center text-sm text-gray-500">
        Checking authentication status...
      </div>
    );
  }

  return (
    <div className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-8">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <Link
            href="/projects"
            className="inline-flex items-center gap-1.5 text-xs font-medium text-emerald-800 hover:text-emerald-900 transition mb-2"
          >
            <ArrowLeft className="w-4 h-4" />
            Back to Public Projects
          </Link>
          <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900 tracking-tight">
            My Community Requests
          </h1>
          <p className="text-sm text-gray-500">
            Track verification progress, coordinator feedback, and project conversion for your neighborhood.
          </p>
        </div>

        <Link
          href="/projects/submit"
          className="inline-flex items-center gap-2 px-4 py-2.5 text-xs font-bold rounded-xl text-emerald-950 bg-emerald-400 hover:bg-emerald-300 shadow-sm transition"
        >
          <PlusCircle className="w-4 h-4" />
          Submit New Request
        </Link>
      </div>

      {error && (
        <div className="p-4 bg-red-50 border border-red-200 rounded-xl flex items-start gap-3">
          <AlertCircle className="w-5 h-5 text-red-600 shrink-0 mt-0.5" />
          <p className="text-sm text-red-800">{error}</p>
        </div>
      )}

      {/* Requests List */}
      {loading ? (
        <div className="p-16 text-center text-gray-500 space-y-3">
          <div className="w-8 h-8 border-4 border-emerald-600 border-t-transparent rounded-full animate-spin mx-auto" />
          <p className="text-sm">Loading your submissions...</p>
        </div>
      ) : requests.length === 0 ? (
        <div className="p-12 bg-white rounded-2xl border border-dashed border-gray-300 text-center space-y-4">
          <FolderKanban className="w-12 h-12 text-gray-400 mx-auto" />
          <div className="space-y-1">
            <h3 className="text-base font-semibold text-gray-900">No requests submitted yet</h3>
            <p className="text-sm text-gray-500 max-w-md mx-auto">
              You haven&apos;t reported any community needs yet. Help improve your local environment by reporting drainage, water, or sanitation issues.
            </p>
          </div>
          <Link
            href="/projects/submit"
            className="inline-flex items-center gap-2 px-5 py-2.5 text-xs font-bold rounded-lg text-emerald-950 bg-emerald-400 hover:bg-emerald-300 shadow-sm transition"
          >
            <PlusCircle className="w-4 h-4" />
            Submit Your First Request
          </Link>
        </div>
      ) : (
        <div className="space-y-4">
          {requests.map((r) => (
            <div
              key={r.id}
              className="p-6 bg-white rounded-2xl border border-gray-200 shadow-sm space-y-4 hover:border-emerald-500/50 transition"
            >
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                <div className="space-y-1">
                  <div className="flex items-center gap-2.5">
                    <span className="text-xs font-mono font-bold text-emerald-800 bg-emerald-50 px-2.5 py-0.5 rounded border border-emerald-200">
                      {r.reference}
                    </span>
                    <span className="text-xs text-gray-400">
                      Submitted {new Date(r.created_at).toLocaleDateString()}
                    </span>
                  </div>
                  <h3 className="text-base font-bold text-gray-900">{r.title}</h3>
                </div>

                <div className="flex items-center gap-2">
                  {getStatusBadge(r.status)}
                </div>
              </div>

              <p className="text-xs text-gray-600 leading-relaxed whitespace-pre-line">
                {r.description}
              </p>

              <div className="flex flex-wrap items-center gap-4 text-xs text-gray-500 pt-2 border-t border-gray-100">
                <div className="flex items-center gap-1.5">
                  <MapPin className="w-3.5 h-3.5 text-gray-400 shrink-0" />
                  <span>
                    {r.community_ward}, {r.lga_name || "LGA"}, {r.state_name || "State"}
                  </span>
                </div>
                <span className="capitalize">Urgency: {r.estimated_urgency}</span>
              </div>

              {/* Feedback Alert for Info Requested */}
              {r.status === "info_requested" && r.info_request_notes && (
                <div className="p-4 bg-amber-50 rounded-xl border border-amber-200 space-y-2">
                  <div className="flex items-center gap-2 text-xs font-bold text-amber-900">
                    <MessageSquare className="w-4 h-4 text-amber-700" />
                    <span>Coordinator Reviewer Requested Additional Information:</span>
                  </div>
                  <p className="text-xs text-amber-800">{r.info_request_notes}</p>
                  <div className="pt-1">
                    <button
                      type="button"
                      onClick={() => handleOpenResubmit(r)}
                      className="inline-flex items-center gap-1.5 px-3.5 py-1.5 text-xs font-bold rounded-lg text-amber-950 bg-amber-300 hover:bg-amber-400 shadow-sm transition"
                    >
                      <RotateCcw className="w-3.5 h-3.5" />
                      Update Details & Resubmit Request
                    </button>
                  </div>
                </div>
              )}

              {/* Feedback Alert for Rejection */}
              {r.status === "rejected" && r.rejection_reason && (
                <div className="p-4 bg-red-50 rounded-xl border border-red-200 space-y-1">
                  <span className="text-xs font-bold text-red-900 block">Documented Reason:</span>
                  <p className="text-xs text-red-800">{r.rejection_reason}</p>
                </div>
              )}

              {/* Converted to Project link */}
              {r.converted_to_project_id && (
                <div className="p-3 bg-emerald-50 rounded-xl border border-emerald-200 flex items-center justify-between text-xs text-emerald-900">
                  <span className="font-semibold">
                    ✓ Converted into Active Community Project!
                  </span>
                  <Link
                    href={`/projects/${r.converted_to_project_id}`}
                    className="inline-flex items-center gap-1 text-xs font-bold text-emerald-800 hover:underline"
                  >
                    View Project
                    <ArrowRight className="w-3.5 h-3.5" />
                  </Link>
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Resubmit Modal */}
      {resubmittingReq && (
        <div className="fixed inset-0 z-50 bg-black/60 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl max-w-lg w-full p-6 space-y-5 shadow-2xl">
            <div className="flex items-center justify-between">
              <h3 className="text-base font-bold text-gray-900">
                Resubmit Request: {resubmittingReq.reference}
              </h3>
              <button
                type="button"
                onClick={() => setResubmittingReq(null)}
                className="p-1 text-gray-400 hover:text-gray-600 rounded-lg"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {resubmitError && (
              <div className="p-3 bg-red-50 text-red-800 rounded-lg text-xs">{resubmitError}</div>
            )}

            <form onSubmit={handleResubmitSubmit} className="space-y-4">
              <div className="space-y-1">
                <label className="text-xs font-bold text-gray-700 block">
                  Updated Description / Clarifications:
                </label>
                <textarea
                  required
                  rows={4}
                  value={resubmitDesc}
                  onChange={(e) => setResubmitDesc(e.target.value)}
                  className="w-full px-3 py-2 text-xs bg-gray-50 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-emerald-500"
                />
              </div>

              <div className="space-y-1">
                <label className="text-xs font-bold text-gray-700 block">
                  Updated Location Details / Landmarks:
                </label>
                <input
                  type="text"
                  required
                  value={resubmitLocation}
                  onChange={(e) => setResubmitLocation(e.target.value)}
                  className="w-full px-3 py-2 text-xs bg-gray-50 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-emerald-500"
                />
              </div>

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setResubmittingReq(null)}
                  className="px-4 py-2 text-xs font-medium text-gray-700 bg-gray-100 hover:bg-gray-200 rounded-lg"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={resubmitLoading}
                  className="px-4 py-2 text-xs font-bold text-white bg-emerald-900 hover:bg-emerald-800 rounded-lg disabled:opacity-50"
                >
                  {resubmitLoading ? "Submitting..." : "Resubmit for Review"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}

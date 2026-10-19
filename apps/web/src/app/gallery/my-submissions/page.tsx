"use client";

import React, { useState, useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  Camera,
  Video,
  Upload,
  Clock,
  CheckCircle,
  XCircle,
  AlertCircle,
  RotateCcw,
  Trash2,
  Calendar,
  MapPin,
  Eye,
  ArrowLeft,
  X,
  ShieldAlert,
} from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import { GalleryMedia, PaginatedGalleryResponse } from "@/lib/types";
import { Spinner } from "@/components/ui/Spinner";
import { Badge } from "@/components/ui/Badge";
import { Alert } from "@/components/ui/Alert";
import { EmptyState } from "@/components/ui/EmptyState";

export default function MySubmissionsPage() {
  const router = useRouter();
  const { user, isLoading } = useAuth();

  const [submissions, setSubmissions] = useState<GalleryMedia[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [page, setPage] = useState<number>(1);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  // Withdraw Modal State
  const [withdrawingItem, setWithdrawingItem] = useState<GalleryMedia | null>(null);
  const [withdrawReason, setWithdrawReason] = useState<string>("");
  const [withdrawing, setWithdrawing] = useState<boolean>(false);
  const [withdrawSuccess, setWithdrawSuccess] = useState<string | null>(null);

  useEffect(() => {
    if (!isLoading && !user) {
      router.push("/login?redirect=" + encodeURIComponent("/gallery/my-submissions"));
    }
  }, [user, isLoading, router]);

  const fetchSubmissions = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await apiRequest<PaginatedGalleryResponse>(`/me/gallery?page=${page}&limit=20`);
      setSubmissions(data.items || []);
      setTotal(data.total || 0);
    } catch (err: any) {
      setError(err.message || "Failed to load your media submissions");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (user) {
      fetchSubmissions();
    }
  }, [user, page]);

  const handleWithdraw = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!withdrawingItem) return;
    setWithdrawing(true);
    try {
      await apiRequest(`/me/gallery/${withdrawingItem.id}/withdraw`, {
        method: "POST",
        body: JSON.stringify({ reason: withdrawReason }),
      });
      setWithdrawSuccess("Submission withdrawn successfully.");
      setTimeout(() => {
        setWithdrawingItem(null);
        setWithdrawSuccess(null);
        setWithdrawReason("");
        fetchSubmissions();
      }, 1500);
    } catch (err: any) {
      setError(err.message || "Failed to withdraw submission");
    } finally {
      setWithdrawing(false);
    }
  };

  const getStatusBadge = (status?: string) => {
    switch (status) {
      case "approved":
        return <Badge variant="success">Approved & Public</Badge>;
      case "pending_review":
        return <Badge variant="warning">Pending Moderation</Badge>;
      case "revision_requested":
        return <Badge variant="primary">Revision Requested</Badge>;
      case "rejected":
        return <Badge variant="danger">Rejected</Badge>;
      case "withdrawn":
        return <Badge variant="neutral">Withdrawn</Badge>;
      case "removed":
        return <Badge variant="neutral">Removed After Publication</Badge>;
      default:
        return <Badge variant="neutral">{status || "Draft"}</Badge>;
    }
  };

  if (isLoading || loading) return <Spinner text="Loading your media submissions..." />;
  if (!user) return null;

  return (
    <div className="min-h-screen bg-gray-50 py-10">
      <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8 space-y-6">
        {/* Navigation Breadcrumb */}
        <Link
          href="/gallery"
          className="inline-flex items-center gap-1.5 text-xs font-semibold text-emerald-800 hover:text-emerald-950 transition"
        >
          <ArrowLeft className="w-4 h-4" />
          <span>Return to Public Gallery</span>
        </Link>

        {/* Header */}
        <div className="bg-white rounded-2xl border border-gray-200 p-6 sm:p-8 shadow-sm flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
          <div className="space-y-1">
            <h1 className="text-2xl font-black text-gray-900 tracking-tight flex items-center gap-2">
              <Camera className="w-6 h-6 text-emerald-800" />
              <span>My Media Submissions</span>
            </h1>
            <p className="text-xs text-gray-500">
              Track moderation status, review coordinator feedback, or withdraw submissions
            </p>
          </div>

          <Link
            href="/gallery/upload"
            className="inline-flex items-center gap-2 px-4 py-2.5 bg-emerald-800 hover:bg-emerald-900 text-white font-bold rounded-xl text-xs transition shadow-sm"
          >
            <Upload className="w-4 h-4" />
            <span>Submit New Media</span>
          </Link>
        </div>

        {error && <Alert type="error">{error}</Alert>}

        {/* Submissions List */}
        {submissions.length === 0 ? (
          <EmptyState
            icon={<Camera className="w-6 h-6" />}
            title="No Submissions Yet"
            description="You haven't uploaded any cleanup photographs or videos yet. Support our movement by uploading authentic media from your community cleanups!"
            actionText="Upload Your First Photo"
            onAction={() => router.push("/gallery/upload")}
          />
        ) : (
          <div className="space-y-4">
            <div className="text-xs text-gray-500 font-medium">
              Showing {submissions.length} of {total} submissions
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {submissions.map((item) => (
                <div
                  key={item.id}
                  className="bg-white rounded-2xl border border-gray-200 p-5 shadow-sm space-y-4 flex flex-col justify-between"
                >
                  <div className="flex gap-4">
                    {/* Media Thumbnail */}
                    <div className="w-24 h-24 rounded-xl bg-gray-900 overflow-hidden flex-shrink-0 flex items-center justify-center relative">
                      {item.thumbnail_url || item.public_url ? (
                        <img
                          src={item.thumbnail_url || item.public_url}
                          alt={item.caption}
                          className="w-full h-full object-cover"
                        />
                      ) : (
                        <div className="text-gray-400 text-xs flex flex-col items-center">
                          {item.media_type === "video" ? <Video className="w-6 h-6" /> : <Camera className="w-6 h-6" />}
                          <span className="text-[9px] mt-1">Quarantined</span>
                        </div>
                      )}
                      {item.media_type === "video" && (
                        <span className="absolute bottom-1 right-1 bg-black/80 text-white text-[9px] px-1 py-0.5 rounded font-bold">
                          VIDEO
                        </span>
                      )}
                    </div>

                    {/* Metadata */}
                    <div className="flex-1 space-y-1.5 min-w-0">
                      <div className="flex items-center gap-2">
                        {getStatusBadge(item.moderation_status)}
                      </div>
                      <h3 className="font-bold text-sm text-gray-900 truncate">
                        {item.caption}
                      </h3>
                      {item.event_title && (
                        <p className="text-xs text-gray-500 truncate flex items-center gap-1">
                          <Calendar className="w-3 h-3 text-emerald-700 flex-shrink-0" />
                          <span>{item.event_title}</span>
                        </p>
                      )}
                      <div className="text-[11px] text-gray-400 flex items-center gap-2 pt-0.5">
                        <span className="flex items-center gap-1">
                          <MapPin className="w-3 h-3 text-emerald-700" />
                          <span>{item.state_name || item.lga_name || "Assigned"}</span>
                        </span>
                        <span>•</span>
                        <span>{new Date(item.created_at).toLocaleDateString("en-GB")}</span>
                      </div>
                    </div>
                  </div>

                  {/* Rejection Reason or Revision Notes Alert */}
                  {item.rejection_reason && (
                    <div className={`p-3 rounded-xl text-xs space-y-1 ${
                      item.moderation_status === "revision_requested"
                        ? "bg-amber-50 border border-amber-200 text-amber-900"
                        : "bg-rose-50 border border-rose-200 text-rose-900"
                    }`}>
                      <div className="font-bold flex items-center gap-1.5">
                        <AlertCircle className="w-3.5 h-3.5" />
                        <span>Moderator Feedback:</span>
                      </div>
                      <p className="leading-relaxed">{item.rejection_reason}</p>
                    </div>
                  )}

                  {/* Actions Bar */}
                  <div className="pt-3 border-t border-gray-100 flex items-center justify-between text-xs">
                    <div>
                      {item.is_published && item.public_url && (
                        <a
                          href={item.public_url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="font-bold text-emerald-700 hover:text-emerald-900 inline-flex items-center gap-1"
                        >
                          <Eye className="w-3.5 h-3.5" />
                          <span>View Public File</span>
                        </a>
                      )}
                    </div>

                    <div className="flex items-center gap-2">
                      {(item.moderation_status === "pending_review" || item.moderation_status === "revision_requested") && (
                        <button
                          type="button"
                          onClick={() => setWithdrawingItem(item)}
                          className="px-3 py-1.5 rounded-lg border border-gray-300 text-gray-700 hover:bg-gray-50 font-medium inline-flex items-center gap-1"
                        >
                          <RotateCcw className="w-3 h-3 text-gray-500" />
                          <span>Withdraw Submission</span>
                        </button>
                      )}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Withdraw Confirmation Modal */}
        {withdrawingItem && (
          <div className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm flex items-center justify-center p-4">
            <div className="bg-white w-full max-w-md rounded-2xl shadow-xl p-6 space-y-4">
              <div className="flex justify-between items-center pb-2 border-b border-gray-100">
                <h3 className="font-bold text-gray-900 text-sm flex items-center gap-2">
                  <RotateCcw className="w-4 h-4 text-amber-600" />
                  Withdraw Media Submission
                </h3>
                <button
                  type="button"
                  onClick={() => setWithdrawingItem(null)}
                  className="text-gray-400 hover:text-gray-600"
                >
                  <X className="w-4 h-4" />
                </button>
              </div>

              {withdrawSuccess && <Alert type="success">{withdrawSuccess}</Alert>}

              {!withdrawSuccess && (
                <form onSubmit={handleWithdraw} className="space-y-3 text-xs">
                  <p className="text-gray-600">
                    Are you sure you want to withdraw &ldquo;{withdrawingItem.caption}&rdquo;? It will be removed from the moderation queue.
                  </p>

                  <div>
                    <label className="block font-semibold text-gray-700 mb-1">Reason for Withdrawal</label>
                    <input
                      type="text"
                      value={withdrawReason}
                      onChange={(e) => setWithdrawReason(e.target.value)}
                      placeholder="e.g. Uploaded incorrect photograph by mistake"
                      className="w-full rounded-lg border-gray-300 text-xs p-2 focus:ring-emerald-600"
                    />
                  </div>

                  <div className="pt-2 flex justify-end gap-2">
                    <button
                      type="button"
                      onClick={() => setWithdrawingItem(null)}
                      className="px-3 py-1.5 rounded-lg border border-gray-300 text-gray-600 hover:bg-gray-50 font-medium"
                    >
                      Cancel
                    </button>
                    <button
                      type="submit"
                      disabled={withdrawing}
                      className="px-4 py-1.5 rounded-lg bg-amber-600 text-white font-bold hover:bg-amber-700 disabled:opacity-50"
                    >
                      {withdrawing ? "Withdrawing..." : "Confirm Withdrawal"}
                    </button>
                  </div>
                </form>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

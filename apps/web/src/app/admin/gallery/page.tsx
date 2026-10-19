"use client";

import React, { useState, useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  Camera,
  Video,
  CheckCircle2,
  XCircle,
  AlertTriangle,
  RotateCcw,
  Trash2,
  Eye,
  EyeOff,
  Filter,
  ArrowLeft,
  Calendar,
  MapPin,
  Clock,
  User,
  Shield,
  Flag,
  Check,
  X,
  FileCheck2,
  AlertCircle,
  FolderKanban,
  ExternalLink,
} from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import {
  GalleryMedia,
  GalleryConsent,
  MediaModerationAction,
  MediaReport,
  MediaTakedownRequest,
  GalleryOverviewStats,
  PaginatedGalleryResponse,
  Zone,
  State,
} from "@/lib/types";
import { Spinner } from "@/components/ui/Spinner";
import { Badge } from "@/components/ui/Badge";
import { Alert } from "@/components/ui/Alert";
import { EmptyState } from "@/components/ui/EmptyState";

type TabType =
  | "overview"
  | "pending_review"
  | "approved"
  | "rejected"
  | "revision_requested"
  | "reports"
  | "takedowns";

export default function AdminGalleryDashboardPage() {
  const router = useRouter();
  const { user, isCoordinator, isLoading } = useAuth();

  const [activeTab, setActiveTab] = useState<TabType>("pending_review");

  // Overview Stats
  const [stats, setStats] = useState<GalleryOverviewStats | null>(null);

  // Gallery Submissions List
  const [mediaList, setMediaList] = useState<GalleryMedia[]>([]);
  const [totalMedia, setTotalMedia] = useState<number>(0);
  const [page, setPage] = useState<number>(1);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  // Geographic filter state
  const [selectedZone, setSelectedZone] = useState<string>("");
  const [selectedState, setSelectedState] = useState<string>("");
  const [zones, setZones] = useState<Zone[]>([]);
  const [states, setStates] = useState<State[]>([]);

  // Detailed Modal State for Review
  const [reviewItem, setReviewItem] = useState<{
    media: GalleryMedia;
    consent: GalleryConsent | null;
    actions: MediaModerationAction[];
  } | null>(null);
  const [reviewLoading, setReviewLoading] = useState<boolean>(false);

  // Moderation Action Inputs
  const [actionReason, setActionReason] = useState<string>("");
  const [actionSubmitting, setActionSubmitting] = useState<boolean>(false);
  const [actionModal, setActionModal] = useState<"reject" | "revision" | "remove" | null>(null);

  // Reports Queue State
  const [reports, setReports] = useState<MediaReport[]>([]);
  const [resolvingReport, setResolvingReport] = useState<MediaReport | null>(null);
  const [reportResolutionStatus, setReportResolutionStatus] = useState<string>("resolved");
  const [reportNotes, setReportNotes] = useState<string>("");

  // Takedowns Queue State
  const [takedowns, setTakedowns] = useState<MediaTakedownRequest[]>([]);
  const [resolvingTakedown, setResolvingTakedown] = useState<MediaTakedownRequest | null>(null);
  const [takedownDecision, setTakedownDecision] = useState<string>("approved_removed");
  const [takedownReason, setTakedownReason] = useState<string>("");

  useEffect(() => {
    if (!isLoading && (!user || !isCoordinator)) {
      router.push("/login?redirect=" + encodeURIComponent("/admin/gallery"));
    }
  }, [user, isCoordinator, isLoading, router]);

  // Load geo dropdowns
  useEffect(() => {
    async function loadGeo() {
      try {
        const [zList, sList] = await Promise.all([
          apiRequest<Zone[]>("/geography/zones"),
          apiRequest<State[]>("/geography/states"),
        ]);
        setZones(zList || []);
        setStates(sList || []);
      } catch (e) {
        console.error("Failed to load geo filters", e);
      }
    }
    loadGeo();
  }, []);

  // Fetch overview stats
  const fetchStats = async () => {
    try {
      const data = await apiRequest<GalleryOverviewStats>("/admin/gallery/overview");
      setStats(data);
    } catch (e) {
      console.error("Failed to load admin stats", e);
    }
  };

  // Fetch data based on active tab
  const fetchData = async () => {
    setLoading(true);
    setError(null);
    try {
      await fetchStats();

      if (activeTab === "reports") {
        const data = await apiRequest<{ items: MediaReport[]; total: number }>(
          `/admin/gallery/reports?page=${page}&limit=20`
        );
        setReports(data.items || []);
        setTotalMedia(data.total || 0);
      } else if (activeTab === "takedowns") {
        const data = await apiRequest<{ items: MediaTakedownRequest[]; total: number }>(
          `/admin/gallery/takedowns?page=${page}&limit=20`
        );
        setTakedowns(data.items || []);
        setTotalMedia(data.total || 0);
      } else if (activeTab !== "overview") {
        const params = new URLSearchParams();
        params.append("status", activeTab);
        if (selectedZone) params.append("zone_id", selectedZone);
        if (selectedState) params.append("state_id", selectedState);
        params.append("page", page.toString());
        params.append("limit", "20");

        const data = await apiRequest<PaginatedGalleryResponse>(`/admin/gallery?${params.toString()}`);
        setMediaList(data.items || []);
        setTotalMedia(data.total || 0);
      }
    } catch (err: any) {
      setError(err.message || "Failed to load moderation queue");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (user && isCoordinator) {
      fetchData();
    }
  }, [user, isCoordinator, activeTab, selectedZone, selectedState, page]);

  // Open detail review modal
  const openReviewModal = async (mediaId: string) => {
    setReviewLoading(true);
    try {
      const data = await apiRequest<{
        media: GalleryMedia;
        consent: GalleryConsent | null;
        actions: MediaModerationAction[];
      }>(`/admin/gallery/${mediaId}`);
      setReviewItem(data);
    } catch (err: any) {
      alert("Failed to load media review details: " + err.message);
    } finally {
      setReviewLoading(false);
    }
  };

  // Moderation Action: Approve
  const handleApprove = async (mediaId: string) => {
    if (!confirm("Are you sure you want to approve this media? It will strip EXIF/GPS metadata, generate public derivatives, and publish it to the public gallery.")) {
      return;
    }
    setActionSubmitting(true);
    try {
      await apiRequest(`/admin/gallery/${mediaId}/approve`, {
        method: "POST",
        body: JSON.stringify({ notes: "Approved and sanitized by coordinator" }),
      });
      setReviewItem(null);
      fetchData();
    } catch (err: any) {
      alert("Approval failed: " + err.message);
    } finally {
      setActionSubmitting(false);
    }
  };

  // Moderation Action: Reject
  const handleReject = async (mediaId: string) => {
    if (!actionReason.trim()) {
      alert("A documented reason for rejection is mandatory.");
      return;
    }
    setActionSubmitting(true);
    try {
      await apiRequest(`/admin/gallery/${mediaId}/reject`, {
        method: "POST",
        body: JSON.stringify({ reason: actionReason.trim() }),
      });
      setActionModal(null);
      setActionReason("");
      setReviewItem(null);
      fetchData();
    } catch (err: any) {
      alert("Rejection failed: " + err.message);
    } finally {
      setActionSubmitting(false);
    }
  };

  // Moderation Action: Request Revision
  const handleRequestRevision = async (mediaId: string) => {
    if (!actionReason.trim()) {
      alert("Revision notes explaining what the uploader must fix are mandatory.");
      return;
    }
    setActionSubmitting(true);
    try {
      await apiRequest(`/admin/gallery/${mediaId}/request-revision`, {
        method: "POST",
        body: JSON.stringify({ notes: actionReason.trim() }),
      });
      setActionModal(null);
      setActionReason("");
      setReviewItem(null);
      fetchData();
    } catch (err: any) {
      alert("Request revision failed: " + err.message);
    } finally {
      setActionSubmitting(false);
    }
  };

  // Moderation Action: Remove Published Media
  const handleRemove = async (mediaId: string) => {
    if (!actionReason.trim()) {
      alert("A clear reason for removing published media is mandatory.");
      return;
    }
    setActionSubmitting(true);
    try {
      await apiRequest(`/admin/gallery/${mediaId}/remove`, {
        method: "POST",
        body: JSON.stringify({ reason: actionReason.trim() }),
      });
      setActionModal(null);
      setActionReason("");
      setReviewItem(null);
      fetchData();
    } catch (err: any) {
      alert("Removal failed: " + err.message);
    } finally {
      setActionSubmitting(false);
    }
  };

  // Moderation Action: Toggle Hide / Unhide
  const handleToggleHide = async (mediaId: string, currentHidden: boolean) => {
    const reason = prompt("Enter documented reason for toggling media visibility:", currentHidden ? "Cleared inspection" : "Under active investigation");
    if (!reason) return;
    try {
      await apiRequest(`/admin/gallery/${mediaId}/toggle-hide`, {
        method: "POST",
        body: JSON.stringify({ is_hidden: !currentHidden, reason }),
      });
      if (reviewItem && reviewItem.media.id === mediaId) {
        setReviewItem({
          ...reviewItem,
          media: { ...reviewItem.media, is_hidden: !currentHidden },
        });
      }
      fetchData();
    } catch (err: any) {
      alert("Toggle hide failed: " + err.message);
    }
  };

  // Resolve Report
  const handleResolveReport = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!resolvingReport) return;
    try {
      await apiRequest(`/admin/gallery/reports/${resolvingReport.id}/resolve`, {
        method: "POST",
        body: JSON.stringify({
          status: reportResolutionStatus,
          notes: reportNotes || "Investigated and resolved by staff",
        }),
      });
      setResolvingReport(null);
      setReportNotes("");
      fetchData();
    } catch (err: any) {
      alert("Resolve report failed: " + err.message);
    }
  };

  // Resolve Takedown
  const handleResolveTakedown = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!resolvingTakedown) return;
    if (!takedownReason.trim()) {
      alert("Resolution reason is required.");
      return;
    }
    try {
      await apiRequest(`/admin/gallery/takedowns/${resolvingTakedown.id}/resolve`, {
        method: "POST",
        body: JSON.stringify({
          decision: takedownDecision,
          reason: takedownReason.trim(),
        }),
      });
      setResolvingTakedown(null);
      setTakedownReason("");
      fetchData();
    } catch (err: any) {
      alert("Resolve takedown failed: " + err.message);
    }
  };

  if (isLoading) return <Spinner text="Authenticating administrator access..." />;
  if (!user || !isCoordinator) return null;

  return (
    <div className="min-h-screen bg-gray-50 py-10">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 space-y-6">
        {/* Navigation Breadcrumb */}
        <Link
          href="/admin"
          className="inline-flex items-center gap-1.5 text-xs font-semibold text-emerald-800 hover:text-emerald-950 transition"
        >
          <ArrowLeft className="w-4 h-4" />
          <span>Return to Admin Workspace</span>
        </Link>

        {/* Dashboard Title & Overview Banner */}
        <div className="bg-emerald-950 text-white rounded-2xl p-6 sm:p-8 shadow-sm flex flex-col md:flex-row justify-between items-start md:items-center gap-6 border border-emerald-900">
          <div className="space-y-1">
            <div className="flex items-center gap-2">
              <span className="text-xs uppercase font-bold tracking-wider text-emerald-400">
                Media Governance Engine
              </span>
              <Badge variant="primary" size="sm">
                PHASE A MODERATION
              </Badge>
            </div>
            <h1 className="text-2xl sm:text-3xl font-black tracking-tight">
              Event Media Gallery & Moderation
            </h1>
            <p className="text-xs text-emerald-200">
              Review volunteer uploads, inspect participant consent declarations, strip private EXIF/GPS metadata, and process takedowns.
            </p>
          </div>

          <div className="flex items-center gap-3">
            <Link
              href="/gallery"
              target="_blank"
              className="px-4 py-2 bg-emerald-800 hover:bg-emerald-700 text-white text-xs font-bold rounded-xl transition inline-flex items-center gap-1.5"
            >
              <span>View Live Gallery</span>
              <ExternalLink className="w-3.5 h-3.5" />
            </Link>
          </div>
        </div>

        {/* Overview Stats Counters */}
        {stats && (
          <div className="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-7 gap-3">
            <div
              onClick={() => setActiveTab("pending_review")}
              className={`p-4 rounded-xl border text-center cursor-pointer transition shadow-sm ${
                activeTab === "pending_review"
                  ? "bg-amber-50 border-amber-400 ring-2 ring-amber-400"
                  : "bg-white border-gray-200 hover:border-amber-300"
              }`}
            >
              <div className="text-xl font-black text-amber-700">{stats.pending_review_count}</div>
              <div className="text-[11px] font-bold text-gray-700 uppercase mt-0.5">Pending Review</div>
            </div>

            <div
              onClick={() => setActiveTab("approved")}
              className={`p-4 rounded-xl border text-center cursor-pointer transition shadow-sm ${
                activeTab === "approved"
                  ? "bg-emerald-50 border-emerald-400 ring-2 ring-emerald-400"
                  : "bg-white border-gray-200 hover:border-emerald-300"
              }`}
            >
              <div className="text-xl font-black text-emerald-700">{stats.approved_count}</div>
              <div className="text-[11px] font-bold text-gray-700 uppercase mt-0.5">Approved & Live</div>
            </div>

            <div
              onClick={() => setActiveTab("revision_requested")}
              className={`p-4 rounded-xl border text-center cursor-pointer transition shadow-sm ${
                activeTab === "revision_requested"
                  ? "bg-blue-50 border-blue-400 ring-2 ring-blue-400"
                  : "bg-white border-gray-200 hover:border-blue-300"
              }`}
            >
              <div className="text-xl font-black text-blue-700">{stats.revision_requested_count}</div>
              <div className="text-[11px] font-bold text-gray-700 uppercase mt-0.5">Revisions</div>
            </div>

            <div
              onClick={() => setActiveTab("rejected")}
              className={`p-4 rounded-xl border text-center cursor-pointer transition shadow-sm ${
                activeTab === "rejected"
                  ? "bg-rose-50 border-rose-400 ring-2 ring-rose-400"
                  : "bg-white border-gray-200 hover:border-rose-300"
              }`}
            >
              <div className="text-xl font-black text-rose-700">{stats.rejected_count}</div>
              <div className="text-[11px] font-bold text-gray-700 uppercase mt-0.5">Rejected</div>
            </div>

            <div
              onClick={() => setActiveTab("reports")}
              className={`p-4 rounded-xl border text-center cursor-pointer transition shadow-sm ${
                activeTab === "reports"
                  ? "bg-orange-50 border-orange-400 ring-2 ring-orange-400"
                  : "bg-white border-gray-200 hover:border-orange-300"
              }`}
            >
              <div className="text-xl font-black text-orange-700">{stats.pending_reports_count}</div>
              <div className="text-[11px] font-bold text-gray-700 uppercase mt-0.5">Reports</div>
            </div>

            <div
              onClick={() => setActiveTab("takedowns")}
              className={`p-4 rounded-xl border text-center cursor-pointer transition shadow-sm ${
                activeTab === "takedowns"
                  ? "bg-purple-50 border-purple-400 ring-2 ring-purple-400"
                  : "bg-white border-gray-200 hover:border-purple-300"
              }`}
            >
              <div className="text-xl font-black text-purple-700">{stats.pending_takedowns_count}</div>
              <div className="text-[11px] font-bold text-gray-700 uppercase mt-0.5">Takedowns</div>
            </div>

            <div className="p-4 rounded-xl border bg-white border-gray-200 text-center shadow-sm">
              <div className="text-xl font-black text-gray-900">{stats.total_media}</div>
              <div className="text-[11px] font-bold text-gray-500 uppercase mt-0.5">Total Uploads</div>
            </div>
          </div>
        )}

        {/* Navigation Tabs */}
        <div className="flex border-b border-gray-200 overflow-x-auto gap-2 text-xs font-bold">
          <button
            type="button"
            onClick={() => {
              setActiveTab("pending_review");
              setPage(1);
            }}
            className={`pb-3 px-3 border-b-2 transition whitespace-nowrap ${
              activeTab === "pending_review"
                ? "border-emerald-800 text-emerald-900"
                : "border-transparent text-gray-500 hover:text-gray-700"
            }`}
          >
            Pending Review ({stats?.pending_review_count || 0})
          </button>
          <button
            type="button"
            onClick={() => {
              setActiveTab("approved");
              setPage(1);
            }}
            className={`pb-3 px-3 border-b-2 transition whitespace-nowrap ${
              activeTab === "approved"
                ? "border-emerald-800 text-emerald-900"
                : "border-transparent text-gray-500 hover:text-gray-700"
            }`}
          >
            Approved & Live ({stats?.approved_count || 0})
          </button>
          <button
            type="button"
            onClick={() => {
              setActiveTab("revision_requested");
              setPage(1);
            }}
            className={`pb-3 px-3 border-b-2 transition whitespace-nowrap ${
              activeTab === "revision_requested"
                ? "border-emerald-800 text-emerald-900"
                : "border-transparent text-gray-500 hover:text-gray-700"
            }`}
          >
            Revisions ({stats?.revision_requested_count || 0})
          </button>
          <button
            type="button"
            onClick={() => {
              setActiveTab("rejected");
              setPage(1);
            }}
            className={`pb-3 px-3 border-b-2 transition whitespace-nowrap ${
              activeTab === "rejected"
                ? "border-emerald-800 text-emerald-900"
                : "border-transparent text-gray-500 hover:text-gray-700"
            }`}
          >
            Rejected ({stats?.rejected_count || 0})
          </button>
          <button
            type="button"
            onClick={() => {
              setActiveTab("reports");
              setPage(1);
            }}
            className={`pb-3 px-3 border-b-2 transition whitespace-nowrap flex items-center gap-1.5 ${
              activeTab === "reports"
                ? "border-emerald-800 text-emerald-900"
                : "border-transparent text-gray-500 hover:text-gray-700"
            }`}
          >
            <Flag className="w-3.5 h-3.5 text-orange-600" />
            <span>Reported Media ({stats?.pending_reports_count || 0})</span>
          </button>
          <button
            type="button"
            onClick={() => {
              setActiveTab("takedowns");
              setPage(1);
            }}
            className={`pb-3 px-3 border-b-2 transition whitespace-nowrap flex items-center gap-1.5 ${
              activeTab === "takedowns"
                ? "border-emerald-800 text-emerald-900"
                : "border-transparent text-gray-500 hover:text-gray-700"
            }`}
          >
            <Shield className="w-3.5 h-3.5 text-purple-600" />
            <span>Takedown Requests ({stats?.pending_takedowns_count || 0})</span>
          </button>
        </div>

        {/* Content Area */}
        {error && <Alert type="error">{error}</Alert>}

        {loading ? (
          <div className="py-20">
            <Spinner text="Loading moderation items..." />
          </div>
        ) : activeTab === "reports" ? (
          /* Reported Media Section */
          <div className="bg-white rounded-2xl border border-gray-200 p-6 shadow-sm space-y-4">
            <h2 className="text-sm font-bold text-gray-900 flex items-center gap-2">
              <Flag className="w-4 h-4 text-orange-600" />
              <span>Public Complaints & Media Concerns</span>
            </h2>

            {reports.length === 0 ? (
              <div className="py-12 text-center text-xs text-gray-500">
                No active complaints reported against gallery media.
              </div>
            ) : (
              <div className="space-y-3">
                {reports.map((rep) => (
                  <div
                    key={rep.id}
                    className="p-4 rounded-xl border border-gray-200 bg-gray-50/50 space-y-3"
                  >
                    <div className="flex justify-between items-start">
                      <div className="space-y-1">
                        <div className="flex items-center gap-2">
                          <Badge variant={rep.status === "pending" ? "danger" : "neutral"} size="sm">
                            {rep.status.toUpperCase()}
                          </Badge>
                          <span className="text-xs font-bold text-gray-900">
                            Reason: {rep.report_reason.replace(/_/g, " ").toUpperCase()}
                          </span>
                        </div>
                        <p className="text-xs text-gray-700 font-medium">{rep.details}</p>
                      </div>

                      <button
                        type="button"
                        onClick={() => setResolvingReport(rep)}
                        className="px-3 py-1.5 bg-emerald-800 text-white rounded-lg text-xs font-bold hover:bg-emerald-900"
                      >
                        Resolve Concern
                      </button>
                    </div>

                    <div className="pt-2 border-t border-gray-200 flex items-center justify-between text-[11px] text-gray-500">
                      <span>Media: {rep.media_caption || rep.media_id}</span>
                      <span>Reported on: {new Date(rep.created_at).toLocaleDateString("en-GB")}</span>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        ) : activeTab === "takedowns" ? (
          /* Takedown Requests Section */
          <div className="bg-white rounded-2xl border border-gray-200 p-6 shadow-sm space-y-4">
            <h2 className="text-sm font-bold text-gray-900 flex items-center gap-2">
              <Shield className="w-4 h-4 text-purple-600" />
              <span>Formal Takedown Requests Queue</span>
            </h2>

            {takedowns.length === 0 ? (
              <div className="py-12 text-center text-xs text-gray-500">
                No formal takedown requests pending in queue.
              </div>
            ) : (
              <div className="space-y-3">
                {takedowns.map((td) => (
                  <div
                    key={td.id}
                    className="p-4 rounded-xl border border-gray-200 bg-gray-50/50 space-y-3"
                  >
                    <div className="flex justify-between items-start">
                      <div className="space-y-1">
                        <div className="flex items-center gap-2">
                          <Badge variant={td.status === "pending" ? "danger" : "neutral"} size="sm">
                            {td.status.toUpperCase()}
                          </Badge>
                          <span className="text-xs font-bold text-gray-900">
                            Requester: {td.requester_name} ({td.requester_email})
                          </span>
                        </div>
                        <p className="text-xs text-gray-600">
                          Relationship: <strong>{td.relationship_to_media}</strong> • Legal grounds: <strong>{td.reason}</strong>
                        </p>
                        <p className="text-xs text-gray-800 font-medium pt-1">&ldquo;{td.details}&rdquo;</p>
                      </div>

                      <button
                        type="button"
                        onClick={() => setResolvingTakedown(td)}
                        className="px-3 py-1.5 bg-purple-700 text-white rounded-lg text-xs font-bold hover:bg-purple-800"
                      >
                        Process Takedown
                      </button>
                    </div>

                    <div className="pt-2 border-t border-gray-200 flex items-center justify-between text-[11px] text-gray-500">
                      <span>Media: {td.media_caption || td.media_id}</span>
                      <span>Filed: {new Date(td.created_at).toLocaleDateString("en-GB")}</span>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        ) : (
          /* Media Moderation Submissions Grid */
          <div className="space-y-4">
            <div className="text-xs text-gray-500 font-medium">
              Showing {mediaList.length} of {totalMedia} records in &ldquo;{activeTab.replace(/_/g, " ")}&rdquo;
            </div>

            {mediaList.length === 0 ? (
              <EmptyState
                icon={<Camera className="w-6 h-6" />}
                title="Queue Empty"
                description={`There are currently no submissions with status '${activeTab}'.`}
              />
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
                {mediaList.map((item) => (
                  <div
                    key={item.id}
                    className="bg-white rounded-2xl border border-gray-200 p-5 shadow-sm space-y-4 flex flex-col justify-between"
                  >
                    <div className="space-y-3">
                      {/* Thumbnail Preview Area */}
                      <div className="relative aspect-video rounded-xl bg-gray-900 overflow-hidden flex items-center justify-center">
                        {item.thumbnail_url || item.public_url ? (
                          <img
                            src={item.thumbnail_url || item.public_url}
                            alt={item.caption}
                            className="w-full h-full object-cover"
                          />
                        ) : (
                          <div className="text-gray-400 text-xs flex flex-col items-center">
                            {item.media_type === "video" ? <Video className="w-8 h-8" /> : <Camera className="w-8 h-8" />}
                            <span className="text-[10px] mt-1 font-semibold">Private Quarantine</span>
                          </div>
                        )}

                        <div className="absolute top-2 left-2">
                          <Badge variant="neutral" size="sm">
                            {item.media_type.toUpperCase()}
                          </Badge>
                        </div>

                        {item.is_hidden && (
                          <div className="absolute top-2 right-2 bg-rose-600 text-white text-[10px] font-bold px-2 py-0.5 rounded shadow">
                            HIDDEN
                          </div>
                        )}
                      </div>

                      {/* Info */}
                      <div className="space-y-1">
                        <h3 className="font-bold text-sm text-gray-900 line-clamp-2">
                          {item.caption}
                        </h3>
                        {item.event_title && (
                          <p className="text-xs text-gray-500 line-clamp-1 flex items-center gap-1">
                            <Calendar className="w-3.5 h-3.5 text-emerald-700" />
                            <span>{item.event_title}</span>
                          </p>
                        )}
                        <p className="text-[11px] text-gray-400 flex items-center gap-1">
                          <MapPin className="w-3.5 h-3.5 text-emerald-700" />
                          <span>{item.state_name || "State"} • {item.lga_name || "LGA"}</span>
                        </p>
                        <p className="text-[11px] text-gray-400">
                          Uploader: {item.uploader_name || "Volunteer"}
                        </p>
                      </div>
                    </div>

                    {/* Moderation Action Buttons */}
                    <div className="pt-3 border-t border-gray-100 flex items-center justify-between gap-2">
                      <button
                        type="button"
                        onClick={() => openReviewModal(item.id)}
                        className="flex-1 py-1.5 px-3 bg-emerald-900 hover:bg-emerald-950 text-white rounded-lg text-xs font-bold transition flex items-center justify-center gap-1"
                      >
                        <Eye className="w-3.5 h-3.5" />
                        <span>Inspect & Review</span>
                      </button>

                      {item.is_published && (
                        <button
                          type="button"
                          onClick={() => handleToggleHide(item.id, item.is_hidden)}
                          title={item.is_hidden ? "Unhide media" : "Temporarily hide media"}
                          className="p-1.5 border border-gray-300 rounded-lg text-gray-600 hover:bg-gray-50"
                        >
                          {item.is_hidden ? <Eye className="w-4 h-4 text-emerald-700" /> : <EyeOff className="w-4 h-4 text-amber-600" />}
                        </button>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}

        {/* Detailed Media Review Inspection Modal */}
        {reviewItem && (
          <div className="fixed inset-0 z-50 bg-black/75 backdrop-blur-sm flex items-center justify-center p-4 overflow-y-auto">
            <div className="bg-white w-full max-w-3xl rounded-2xl shadow-2xl p-6 sm:p-8 space-y-6 my-auto max-h-[92vh] overflow-y-auto border border-gray-200">
              <div className="flex justify-between items-start pb-3 border-b border-gray-200">
                <div className="space-y-0.5">
                  <span className="text-[10px] uppercase font-bold text-emerald-800 tracking-wider">
                    Moderator Inspection Workspace
                  </span>
                  <h2 className="text-lg font-black text-gray-900">{reviewItem.media.caption}</h2>
                </div>
                <button
                  type="button"
                  onClick={() => setReviewItem(null)}
                  className="text-gray-400 hover:text-gray-600 p-1"
                >
                  <X className="w-5 h-5" />
                </button>
              </div>

              {/* Media Preview Box */}
              <div className="aspect-video bg-black rounded-xl overflow-hidden flex items-center justify-center">
                {reviewItem.media.media_type === "video" ? (
                  <video
                    src={reviewItem.media.public_url || undefined}
                    controls
                    className="w-full h-full object-contain"
                  />
                ) : (
                  <img
                    src={reviewItem.media.public_url || reviewItem.media.thumbnail_url || undefined}
                    alt={reviewItem.media.caption}
                    className="w-full h-full object-contain"
                  />
                )}
              </div>

              {/* Event & Jurisdiction Attribution */}
              <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs bg-gray-50 p-3.5 rounded-xl border border-gray-200">
                <div>
                  <span className="text-gray-400 block text-[10px] uppercase font-bold">Event</span>
                  <span className="font-semibold text-gray-900">{reviewItem.media.event_title}</span>
                </div>
                <div>
                  <span className="text-gray-400 block text-[10px] uppercase font-bold">State & LGA</span>
                  <span className="font-semibold text-gray-900">{reviewItem.media.state_name} • {reviewItem.media.lga_name}</span>
                </div>
                <div>
                  <span className="text-gray-400 block text-[10px] uppercase font-bold">Uploader</span>
                  <span className="font-semibold text-gray-900">{reviewItem.media.uploader_name}</span>
                </div>
                <div>
                  <span className="text-gray-400 block text-[10px] uppercase font-bold">Status</span>
                  <span className="font-semibold text-gray-900">{reviewItem.media.moderation_status?.toUpperCase()}</span>
                </div>
              </div>

              {/* Consent Checklist */}
              <div className="space-y-2">
                <h4 className="text-xs font-bold text-gray-900 uppercase tracking-wider flex items-center gap-1.5">
                  <FileCheck2 className="w-4 h-4 text-emerald-800" />
                  <span>Participant & Safeguarding Consent Record</span>
                </h4>

                {reviewItem.consent ? (
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 text-xs">
                    <div className="flex items-center gap-2 p-2 bg-gray-50 rounded-lg">
                      <Check className="w-4 h-4 text-emerald-700" />
                      <span>Owner Permission Confirmed</span>
                    </div>
                    <div className="flex items-center gap-2 p-2 bg-gray-50 rounded-lg">
                      <Check className="w-4 h-4 text-emerald-700" />
                      <span>Public Display Understood</span>
                    </div>
                    <div className="flex items-center gap-2 p-2 bg-gray-50 rounded-lg">
                      <Check className="w-4 h-4 text-emerald-700" />
                      <span>Identifiable Subjects Consented</span>
                    </div>
                    <div className="flex items-center gap-2 p-2 bg-gray-50 rounded-lg">
                      <Check className="w-4 h-4 text-emerald-700" />
                      <span>No Confidential Data Disclosed</span>
                    </div>
                    {reviewItem.consent.has_minors && (
                      <div className="col-span-1 sm:col-span-2 p-2.5 bg-amber-50 border border-amber-200 rounded-lg text-amber-950 font-semibold flex items-center gap-2">
                        <AlertTriangle className="w-4 h-4 text-amber-600 flex-shrink-0" />
                        <span>Minors Pictured: Verified Guardian Consent Declared ({reviewItem.consent.has_guardian_consent ? "YES" : "NO"})</span>
                      </div>
                    )}
                  </div>
                ) : (
                  <p className="text-xs text-gray-400">Consent record not available.</p>
                )}
              </div>

              {/* Moderation Action Decision Buttons */}
              <div className="pt-4 border-t border-gray-200 flex flex-wrap items-center justify-between gap-3">
                <div className="flex items-center gap-2">
                  {reviewItem.media.moderation_status !== "approved" && (
                    <button
                      type="button"
                      disabled={actionSubmitting}
                      onClick={() => handleApprove(reviewItem.media.id)}
                      className="px-4 py-2 bg-emerald-800 hover:bg-emerald-900 text-white rounded-xl text-xs font-bold transition flex items-center gap-1.5 shadow-sm disabled:opacity-50"
                    >
                      <Check className="w-4 h-4" />
                      <span>Approve & Publish (Sanitize)</span>
                    </button>
                  )}

                  {reviewItem.media.moderation_status !== "rejected" && (
                    <button
                      type="button"
                      onClick={() => setActionModal("reject")}
                      className="px-4 py-2 bg-rose-700 hover:bg-rose-800 text-white rounded-xl text-xs font-bold transition flex items-center gap-1.5 shadow-sm"
                    >
                      <X className="w-4 h-4" />
                      <span>Reject</span>
                    </button>
                  )}

                  <button
                    type="button"
                    onClick={() => setActionModal("revision")}
                    className="px-3.5 py-2 bg-amber-600 hover:bg-amber-700 text-white rounded-xl text-xs font-bold transition flex items-center gap-1.5 shadow-sm"
                  >
                    <RotateCcw className="w-4 h-4" />
                    <span>Request Revision</span>
                  </button>
                </div>

                {reviewItem.media.is_published && (
                  <button
                    type="button"
                    onClick={() => setActionModal("remove")}
                    className="px-3.5 py-2 bg-gray-900 hover:bg-black text-white rounded-xl text-xs font-bold transition flex items-center gap-1.5 shadow-sm"
                  >
                    <Trash2 className="w-4 h-4 text-rose-400" />
                    <span>Evict & Remove</span>
                  </button>
                )}
              </div>
            </div>
          </div>
        )}

        {/* Action Reason Input Modal (Reject / Revision / Remove) */}
        {actionModal && reviewItem && (
          <div className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm flex items-center justify-center p-4">
            <div className="bg-white w-full max-w-md rounded-2xl shadow-xl p-6 space-y-4">
              <h3 className="text-sm font-bold text-gray-900 uppercase tracking-wider">
                {actionModal === "reject" && "Document Reason for Rejection"}
                {actionModal === "revision" && "Specify Required Revisions"}
                {actionModal === "remove" && "Document Reason for Permanent Removal"}
              </h3>

              <p className="text-xs text-gray-500">
                This documented note will be preserved in the immutable audit log and shared with the uploader.
              </p>

              <textarea
                required
                rows={3}
                value={actionReason}
                onChange={(e) => setActionReason(e.target.value)}
                placeholder="e.g. Photograph contains blurry artifacts / missing clear guardian consent disclosure..."
                className="w-full rounded-xl border-gray-300 text-xs p-3 focus:ring-emerald-600"
              />

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => {
                    setActionModal(null);
                    setActionReason("");
                  }}
                  className="px-3 py-1.5 border border-gray-300 rounded-lg text-xs font-semibold text-gray-600 hover:bg-gray-50"
                >
                  Cancel
                </button>

                {actionModal === "reject" && (
                  <button
                    type="button"
                    disabled={actionSubmitting}
                    onClick={() => handleReject(reviewItem.media.id)}
                    className="px-4 py-1.5 bg-rose-700 text-white rounded-lg text-xs font-bold hover:bg-rose-800 disabled:opacity-50"
                  >
                    Confirm Rejection
                  </button>
                )}
                {actionModal === "revision" && (
                  <button
                    type="button"
                    disabled={actionSubmitting}
                    onClick={() => handleRequestRevision(reviewItem.media.id)}
                    className="px-4 py-1.5 bg-amber-600 text-white rounded-lg text-xs font-bold hover:bg-amber-700 disabled:opacity-50"
                  >
                    Send Revision Request
                  </button>
                )}
                {actionModal === "remove" && (
                  <button
                    type="button"
                    disabled={actionSubmitting}
                    onClick={() => handleRemove(reviewItem.media.id)}
                    className="px-4 py-1.5 bg-gray-950 text-white rounded-lg text-xs font-bold hover:black disabled:opacity-50"
                  >
                    Confirm Eviction
                  </button>
                )}
              </div>
            </div>
          </div>
        )}

        {/* Resolve Report Modal */}
        {resolvingReport && (
          <div className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm flex items-center justify-center p-4">
            <div className="bg-white w-full max-w-md rounded-2xl shadow-xl p-6 space-y-4">
              <h3 className="font-bold text-sm text-gray-900">Resolve Reported Media Concern</h3>
              <form onSubmit={handleResolveReport} className="space-y-3 text-xs">
                <div>
                  <label className="block font-semibold text-gray-700 mb-1">Decision</label>
                  <select
                    value={reportResolutionStatus}
                    onChange={(e) => setReportResolutionStatus(e.target.value)}
                    className="w-full rounded-lg border-gray-300 text-xs py-2 px-2.5 bg-gray-50"
                  >
                    <option value="resolved">Resolved (Action taken / Issue addressed)</option>
                    <option value="dismissed">Dismissed (No violation found)</option>
                    <option value="investigating">Keep Investigating</option>
                  </select>
                </div>

                <div>
                  <label className="block font-semibold text-gray-700 mb-1">Investigation Notes *</label>
                  <textarea
                    required
                    rows={3}
                    value={reportNotes}
                    onChange={(e) => setReportNotes(e.target.value)}
                    placeholder="Document outcome of review..."
                    className="w-full rounded-lg border-gray-300 text-xs p-2.5"
                  />
                </div>

                <div className="flex justify-end gap-2 pt-2">
                  <button
                    type="button"
                    onClick={() => setResolvingReport(null)}
                    className="px-3 py-1.5 border border-gray-300 rounded-lg font-semibold text-gray-600"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    className="px-4 py-1.5 bg-emerald-800 text-white rounded-lg font-bold hover:bg-emerald-900"
                  >
                    Confirm Resolution
                  </button>
                </div>
              </form>
            </div>
          </div>
        )}

        {/* Resolve Takedown Modal */}
        {resolvingTakedown && (
          <div className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm flex items-center justify-center p-4">
            <div className="bg-white w-full max-w-md rounded-2xl shadow-xl p-6 space-y-4">
              <h3 className="font-bold text-sm text-gray-900">Process Formal Takedown Request</h3>
              <form onSubmit={handleResolveTakedown} className="space-y-3 text-xs">
                <div>
                  <label className="block font-semibold text-gray-700 mb-1">Decision</label>
                  <select
                    value={takedownDecision}
                    onChange={(e) => setTakedownDecision(e.target.value)}
                    className="w-full rounded-lg border-gray-300 text-xs py-2 px-2.5 bg-gray-50"
                  >
                    <option value="approved_removed">Approve & Evict Media (Remove from R2 & Gallery)</option>
                    <option value="rejected_retained">Reject Request (Retain Media)</option>
                  </select>
                </div>

                <div>
                  <label className="block font-semibold text-gray-700 mb-1">Documented Legal Reason *</label>
                  <textarea
                    required
                    rows={3}
                    value={takedownReason}
                    onChange={(e) => setTakedownReason(e.target.value)}
                    placeholder="State legal or policy basis for this takedown decision..."
                    className="w-full rounded-lg border-gray-300 text-xs p-2.5"
                  />
                </div>

                <div className="flex justify-end gap-2 pt-2">
                  <button
                    type="button"
                    onClick={() => setResolvingTakedown(null)}
                    className="px-3 py-1.5 border border-gray-300 rounded-lg font-semibold text-gray-600"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    className="px-4 py-1.5 bg-purple-700 text-white rounded-lg font-bold hover:bg-purple-800"
                  >
                    Execute Takedown Decision
                  </button>
                </div>
              </form>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

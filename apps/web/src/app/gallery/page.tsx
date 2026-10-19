"use client";

import React, { useEffect, useState, useMemo } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import Image from "next/image";
import {
  Camera,
  Video,
  Filter,
  Calendar,
  MapPin,
  Clock,
  User,
  Shield,
  Eye,
  Flag,
  X,
  Upload,
  Play,
  Share2,
  ChevronLeft,
  ChevronRight,
  AlertTriangle,
  CheckCircle,
  FileText,
} from "lucide-react";
import { apiRequest } from "@/lib/api";
import { GalleryMedia, PaginatedGalleryResponse, Zone, State, LGA } from "@/lib/types";
import { Spinner } from "@/components/ui/Spinner";
import { Badge } from "@/components/ui/Badge";
import { EmptyState } from "@/components/ui/EmptyState";
import { Alert } from "@/components/ui/Alert";
import { useAuth } from "@/lib/auth-context";

export default function PublicGalleryPage() {
  const router = useRouter();
  const { user } = useAuth();

  // Filter States
  const [mediaType, setMediaType] = useState<string>("all");
  const [selectedZone, setSelectedZone] = useState<string>("");
  const [selectedState, setSelectedState] = useState<string>("");
  const [selectedLga, setSelectedLga] = useState<string>("");
  const [selectedYear, setSelectedYear] = useState<string>("");
  const [page, setPage] = useState<number>(1);
  const limit = 24;

  // Metadata for filter dropdowns
  const [zones, setZones] = useState<Zone[]>([]);
  const [states, setStates] = useState<State[]>([]);
  const [lgas, setLgas] = useState<LGA[]>([]);

  // Gallery Data
  const [galleryData, setGalleryData] = useState<PaginatedGalleryResponse | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  // Active Lightbox / Media Detail Modal
  const [activeMedia, setActiveMedia] = useState<GalleryMedia | null>(null);

  // Report Modal
  const [reportingMedia, setReportingMedia] = useState<GalleryMedia | null>(null);
  const [reportReason, setReportReason] = useState<string>("privacy_violation");
  const [reportEmail, setReportEmail] = useState<string>("");
  const [reportDetails, setReportDetails] = useState<string>("");
  const [reportSubmitting, setReportSubmitting] = useState<boolean>(false);
  const [reportSuccess, setReportSuccess] = useState<string | null>(null);
  const [reportError, setReportError] = useState<string | null>(null);

  // Formal Takedown Modal
  const [takedownMedia, setTakedownMedia] = useState<GalleryMedia | null>(null);
  const [takedownName, setTakedownName] = useState<string>("");
  const [takedownEmail, setTakedownEmail] = useState<string>("");
  const [takedownRelation, setTakedownRelation] = useState<string>("uploader");
  const [takedownReason, setTakedownReason] = useState<string>("privacy_violation");
  const [takedownDetails, setTakedownDetails] = useState<string>("");
  const [takedownSubmitting, setTakedownSubmitting] = useState<boolean>(false);
  const [takedownSuccess, setTakedownSuccess] = useState<string | null>(null);
  const [takedownError, setTakedownError] = useState<string | null>(null);

  // Load Geographic metadata for filters
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
        console.error("Failed to load geographic filter metadata", e);
      }
    }
    loadGeo();
  }, []);

  // When state changes, fetch its LGAs
  useEffect(() => {
    if (!selectedState) {
      setLgas([]);
      setSelectedLga("");
      return;
    }
    async function loadLgas() {
      try {
        const data = await apiRequest<LGA[]>(`/geography/states/${selectedState}/lgas`);
        setLgas(data || []);
      } catch (e) {
        console.error("Failed to load LGAs", e);
      }
    }
    loadLgas();
  }, [selectedState]);

  // Fetch Public Gallery Media with backend query parameters
  useEffect(() => {
    async function fetchGallery() {
      setLoading(true);
      setError(null);
      try {
        const params = new URLSearchParams();
        if (mediaType !== "all") params.append("type", mediaType);
        if (selectedZone) params.append("zone_id", selectedZone);
        if (selectedState) params.append("state_id", selectedState);
        if (selectedLga) params.append("lga_id", selectedLga);
        if (selectedYear) params.append("year", selectedYear);
        params.append("page", page.toString());
        params.append("limit", limit.toString());

        const res = await apiRequest<PaginatedGalleryResponse>(`/gallery?${params.toString()}`);
        setGalleryData(res);
      } catch (err: any) {
        setError(err.message || "Failed to load public media gallery");
      } finally {
        setLoading(false);
      }
    }
    fetchGallery();
  }, [mediaType, selectedZone, selectedState, selectedLga, selectedYear, page]);

  // Filtered states by zone
  const filteredStates = useMemo(() => {
    if (!selectedZone) return states;
    return states.filter((s) => s.zone_id === selectedZone);
  }, [states, selectedZone]);

  const totalPages = galleryData ? Math.ceil(galleryData.total / limit) : 1;

  // Handle Report Submission
  const handleReportSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!reportingMedia) return;
    setReportSubmitting(true);
    setReportError(null);
    try {
      await apiRequest(`/gallery/${reportingMedia.id}/report`, {
        method: "POST",
        body: JSON.stringify({
          reporter_email: reportEmail || undefined,
          report_reason: reportReason,
          details: reportDetails,
        }),
      });
      setReportSuccess("Report submitted successfully. Our safeguarding moderators will review this media.");
      setTimeout(() => {
        setReportingMedia(null);
        setReportSuccess(null);
        setReportDetails("");
      }, 2000);
    } catch (err: any) {
      setReportError(err.message || "Failed to submit report");
    } finally {
      setReportSubmitting(false);
    }
  };

  // Handle Takedown Submission
  const handleTakedownSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!takedownMedia) return;
    setTakedownSubmitting(true);
    setTakedownError(null);
    try {
      await apiRequest(`/gallery/${takedownMedia.id}/takedown`, {
        method: "POST",
        body: JSON.stringify({
          requester_name: takedownName,
          requester_email: takedownEmail,
          relationship_to_media: takedownRelation,
          reason: takedownReason,
          details: takedownDetails,
        }),
      });
      setTakedownSuccess("Takedown request formally submitted. A review officer will process your request promptly.");
      setTimeout(() => {
        setTakedownMedia(null);
        setTakedownSuccess(null);
        setTakedownName("");
        setTakedownEmail("");
        setTakedownDetails("");
      }, 2000);
    } catch (err: any) {
      setTakedownError(err.message || "Failed to submit takedown request");
    } finally {
      setTakedownSubmitting(false);
    }
  };

  return (
    <div className="min-h-screen bg-gray-50 pb-20">
      {/* Hero Header */}
      <section className="bg-emerald-950 text-white py-14 border-b border-emerald-900">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-6">
            <div className="space-y-3 max-w-2xl">
              <div className="inline-flex items-center gap-2 px-3 py-1 bg-emerald-900 text-emerald-300 rounded-full text-xs font-semibold uppercase tracking-wider">
                <Camera className="w-3.5 h-3.5" />
                <span>Verified Cleanups In Action</span>
              </div>
              <h1 className="text-3xl sm:text-4xl font-extrabold tracking-tight">
                Movement Media Gallery
              </h1>
              <p className="text-sm sm:text-base text-emerald-100/90 leading-relaxed">
                Explore sanitized and approved photographs and videos documenting Nigerian Ratels volunteer cleanup operations, community sanitation interventions, and civic stewardship across all 36 States and the FCT.
              </p>
            </div>

            <div className="flex flex-wrap items-center gap-3">
              <Link
                href="/gallery/upload"
                className="inline-flex items-center gap-2 px-5 py-2.5 bg-emerald-500 hover:bg-emerald-400 text-emerald-950 font-bold rounded-xl text-sm transition shadow-sm"
              >
                <Upload className="w-4 h-4" />
                <span>Upload Event Media</span>
              </Link>
              {user && (
                <Link
                  href="/gallery/my-submissions"
                  className="inline-flex items-center gap-2 px-4 py-2.5 bg-emerald-900 hover:bg-emerald-800 text-white font-medium rounded-xl text-sm transition border border-emerald-700"
                >
                  <Eye className="w-4 h-4" />
                  <span>My Submissions</span>
                </Link>
              )}
            </div>
          </div>
        </div>
      </section>

      {/* Main Content Area */}
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 pt-8 space-y-6">
        {/* Multi-Dimensional Filter Bar */}
        <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-4">
          <div className="flex items-center gap-2 pb-2 border-b border-gray-100 text-xs font-bold text-gray-700 uppercase tracking-wider">
            <Filter className="w-4 h-4 text-emerald-700" />
            <span>Search & Geographic Filtering</span>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-3 text-xs">
            {/* Media Type Filter */}
            <div>
              <label className="block text-gray-600 font-semibold mb-1">Media Type</label>
              <select
                value={mediaType}
                onChange={(e) => {
                  setMediaType(e.target.value);
                  setPage(1);
                }}
                className="w-full rounded-lg border-gray-300 text-xs py-2 px-2.5 focus:ring-emerald-600 focus:border-emerald-600 bg-gray-50"
              >
                <option value="all">All Media (Photos & Videos)</option>
                <option value="image">Photographs Only</option>
                <option value="video">Videos Only</option>
              </select>
            </div>

            {/* Year Filter */}
            <div>
              <label className="block text-gray-600 font-semibold mb-1">Year</label>
              <select
                value={selectedYear}
                onChange={(e) => {
                  setSelectedYear(e.target.value);
                  setPage(1);
                }}
                className="w-full rounded-lg border-gray-300 text-xs py-2 px-2.5 focus:ring-emerald-600 focus:border-emerald-600 bg-gray-50"
              >
                <option value="">All Years</option>
                <option value="2026">2026</option>
                <option value="2025">2025</option>
                <option value="2024">2024</option>
              </select>
            </div>

            {/* Geopolitical Zone */}
            <div>
              <label className="block text-gray-600 font-semibold mb-1">Geopolitical Zone</label>
              <select
                value={selectedZone}
                onChange={(e) => {
                  setSelectedZone(e.target.value);
                  setSelectedState("");
                  setSelectedLga("");
                  setPage(1);
                }}
                className="w-full rounded-lg border-gray-300 text-xs py-2 px-2.5 focus:ring-emerald-600 focus:border-emerald-600 bg-gray-50"
              >
                <option value="">All Zones</option>
                {zones.map((z) => (
                  <option key={z.id} value={z.id}>
                    {z.name}
                  </option>
                ))}
              </select>
            </div>

            {/* State */}
            <div>
              <label className="block text-gray-600 font-semibold mb-1">State / FCT</label>
              <select
                value={selectedState}
                onChange={(e) => {
                  setSelectedState(e.target.value);
                  setSelectedLga("");
                  setPage(1);
                }}
                className="w-full rounded-lg border-gray-300 text-xs py-2 px-2.5 focus:ring-emerald-600 focus:border-emerald-600 bg-gray-50"
              >
                <option value="">All States</option>
                {filteredStates.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
                  </option>
                ))}
              </select>
            </div>

            {/* LGA */}
            <div>
              <label className="block text-gray-600 font-semibold mb-1">Local Government (LGA)</label>
              <select
                value={selectedLga}
                disabled={!selectedState}
                onChange={(e) => {
                  setSelectedLga(e.target.value);
                  setPage(1);
                }}
                className="w-full rounded-lg border-gray-300 text-xs py-2 px-2.5 focus:ring-emerald-600 focus:border-emerald-600 bg-gray-50 disabled:bg-gray-100 disabled:text-gray-400"
              >
                <option value="">All LGAs</option>
                {lgas.map((l) => (
                  <option key={l.id} value={l.id}>
                    {l.name}
                  </option>
                ))}
              </select>
            </div>
          </div>

          {/* Reset Filters Shortcut */}
          {(mediaType !== "all" || selectedZone || selectedState || selectedLga || selectedYear) && (
            <div className="flex justify-between items-center pt-2 text-xs text-gray-500">
              <span>Active filters applied</span>
              <button
                type="button"
                onClick={() => {
                  setMediaType("all");
                  setSelectedZone("");
                  setSelectedState("");
                  setSelectedLga("");
                  setSelectedYear("");
                  setPage(1);
                }}
                className="text-emerald-700 hover:text-emerald-900 font-bold underline"
              >
                Reset All Filters
              </button>
            </div>
          )}
        </div>

        {/* Gallery Content */}
        {error && <Alert type="error">{error}</Alert>}

        {loading ? (
          <div className="py-24">
            <Spinner text="Loading approved event gallery..." />
          </div>
        ) : !galleryData || galleryData.items.length === 0 ? (
          <EmptyState
            icon={<Camera className="w-6 h-6" />}
            title="No Approved Media Found"
            description="There are currently no approved photographs or videos matching your filter criteria. Be the first to upload media from your cleanup venue!"
            actionText="Submit Cleanup Media"
            onAction={() => router.push("/gallery/upload")}
          />
        ) : (
          <div className="space-y-8">
            <div className="flex justify-between items-center text-xs text-gray-500 font-medium">
              <span>Showing {galleryData.items.length} of {galleryData.total} verified media items</span>
              <span>Page {galleryData.page} of {totalPages}</span>
            </div>

            {/* Responsive Media Grid */}
            <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-6">
              {galleryData.items.map((item) => {
                const isVideo = item.media_type === "video";
                const displayUrl = item.thumbnail_url || item.public_url || "";

                return (
                  <div
                    key={item.id}
                    onClick={() => setActiveMedia(item)}
                    className="group bg-white rounded-2xl border border-gray-200 overflow-hidden shadow-sm hover:shadow-md transition cursor-pointer flex flex-col"
                  >
                    {/* Media Thumbnail Container with 4:3 Aspect Ratio */}
                    <div className="relative w-full aspect-[4/3] bg-gray-900 overflow-hidden">
                      {displayUrl ? (
                        isVideo && !item.thumbnail_url ? (
                          <video
                            src={displayUrl}
                            preload="metadata"
                            className="w-full h-full object-cover group-hover:scale-105 transition duration-300"
                          />
                        ) : (
                          <img
                            src={displayUrl}
                            alt={item.caption}
                            loading="lazy"
                            className="w-full h-full object-cover group-hover:scale-105 transition duration-300"
                          />
                        )
                      ) : (
                        <div className="w-full h-full flex items-center justify-center text-gray-400">
                          <Camera className="w-8 h-8" />
                        </div>
                      )}

                      {/* Video Indicator Badge */}
                      {isVideo && (
                        <div className="absolute top-3 left-3 bg-black/70 backdrop-blur-md text-white px-2 py-1 rounded-md text-[10px] font-bold flex items-center gap-1 shadow-sm">
                          <Play className="w-3 h-3 fill-current text-emerald-400" />
                          <span>VIDEO</span>
                        </div>
                      )}

                      {/* Featured Star Badge */}
                      {item.is_featured && (
                        <div className="absolute top-3 right-3 bg-amber-500 text-emerald-950 px-2 py-0.5 rounded-md text-[10px] font-black uppercase tracking-wider shadow-sm">
                          Featured
                        </div>
                      )}

                      {/* Hover Overlay */}
                      <div className="absolute inset-0 bg-emerald-950/40 opacity-0 group-hover:opacity-100 transition flex items-center justify-center">
                        <div className="bg-white/90 backdrop-blur text-emerald-950 px-3 py-1.5 rounded-lg text-xs font-bold flex items-center gap-1.5 shadow-lg">
                          <Eye className="w-4 h-4 text-emerald-700" />
                          <span>View Media</span>
                        </div>
                      </div>
                    </div>

                    {/* Metadata Content */}
                    <div className="p-4 flex-1 flex flex-col justify-between space-y-3">
                      <div className="space-y-1.5">
                        <h3 className="font-bold text-gray-900 text-sm line-clamp-2 leading-snug group-hover:text-emerald-800 transition">
                          {item.caption}
                        </h3>
                        {item.event_title && (
                          <p className="text-xs text-gray-500 line-clamp-1 flex items-center gap-1">
                            <Calendar className="w-3 h-3 text-emerald-700 flex-shrink-0" />
                            <span>{item.event_title}</span>
                          </p>
                        )}
                      </div>

                      <div className="pt-2 border-t border-gray-100 flex items-center justify-between text-[11px] text-gray-500">
                        <span className="flex items-center gap-1 font-medium text-emerald-900">
                          <MapPin className="w-3 h-3 text-emerald-700" />
                          <span>{item.state_name || item.lga_name || "Nigeria"}</span>
                        </span>
                        {item.published_at && (
                          <span>
                            {new Date(item.published_at).toLocaleDateString("en-GB", {
                              day: "numeric",
                              month: "short",
                              year: "numeric",
                            })}
                          </span>
                        )}
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>

            {/* Pagination Controls */}
            {totalPages > 1 && (
              <div className="flex justify-center items-center gap-2 pt-6">
                <button
                  type="button"
                  disabled={page <= 1}
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                  className="p-2 border border-gray-300 rounded-lg text-xs font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-40 disabled:cursor-not-allowed"
                >
                  <ChevronLeft className="w-4 h-4" />
                </button>
                <span className="text-xs font-semibold text-gray-700 px-3">
                  Page {page} of {totalPages}
                </span>
                <button
                  type="button"
                  disabled={page >= totalPages}
                  onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                  className="p-2 border border-gray-300 rounded-lg text-xs font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-40 disabled:cursor-not-allowed"
                >
                  <ChevronRight className="w-4 h-4" />
                </button>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Lightbox / Media Detail Modal */}
      {activeMedia && (
        <div className="fixed inset-0 z-50 bg-black/85 backdrop-blur-sm flex items-center justify-center p-4 sm:p-6 overflow-y-auto">
          <div className="bg-white w-full max-w-4xl rounded-2xl overflow-hidden shadow-2xl relative my-auto border border-gray-800">
            {/* Close Button */}
            <button
              type="button"
              onClick={() => setActiveMedia(null)}
              className="absolute top-4 right-4 z-20 bg-black/60 hover:bg-black/90 text-white p-2 rounded-full transition"
            >
              <X className="w-5 h-5" />
            </button>

            {/* Media Player / Image Area */}
            <div className="bg-black flex items-center justify-center min-h-[300px] max-h-[70vh] overflow-hidden">
              {activeMedia.media_type === "video" ? (
                <video
                  src={activeMedia.public_url}
                  controls
                  autoPlay={false}
                  className="w-full max-h-[68vh] object-contain"
                />
              ) : (
                <img
                  src={activeMedia.public_url || activeMedia.thumbnail_url}
                  alt={activeMedia.caption}
                  className="w-full max-h-[68vh] object-contain"
                />
              )}
            </div>

            {/* Media Details Footer */}
            <div className="p-6 space-y-4">
              <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
                <div className="space-y-1">
                  <h2 className="text-lg font-bold text-gray-900 leading-snug">
                    {activeMedia.caption}
                  </h2>
                  {activeMedia.description && (
                    <p className="text-xs text-gray-600 leading-relaxed max-w-2xl">
                      {activeMedia.description}
                    </p>
                  )}
                </div>

                {/* Report and Takedown Actions */}
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    onClick={() => {
                      setReportingMedia(activeMedia);
                    }}
                    className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-gray-300 text-xs font-semibold text-gray-700 hover:bg-gray-50 transition"
                  >
                    <Flag className="w-3.5 h-3.5 text-amber-600" />
                    <span>Report Concern</span>
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      setTakedownMedia(activeMedia);
                    }}
                    className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-gray-300 text-xs font-semibold text-rose-700 hover:bg-rose-50 transition"
                  >
                    <Shield className="w-3.5 h-3.5 text-rose-600" />
                    <span>Request Takedown</span>
                  </button>
                </div>
              </div>

              {/* Attribution and Location Metadata */}
              <div className="pt-3 border-t border-gray-100 grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs text-gray-600">
                <div>
                  <span className="text-gray-400 block text-[10px] uppercase font-bold">Event</span>
                  <span className="font-semibold text-gray-900 line-clamp-1">{activeMedia.event_title || "Cleanup Activity"}</span>
                </div>
                <div>
                  <span className="text-gray-400 block text-[10px] uppercase font-bold">Location</span>
                  <span className="font-semibold text-gray-900">{activeMedia.lga_name ? `${activeMedia.lga_name}, ` : ""}{activeMedia.state_name || "Nigeria"}</span>
                </div>
                <div>
                  <span className="text-gray-400 block text-[10px] uppercase font-bold">Publication Date</span>
                  <span className="font-semibold text-gray-900">
                    {activeMedia.published_at ? new Date(activeMedia.published_at).toLocaleDateString("en-GB", {
                      day: "numeric",
                      month: "short",
                      year: "numeric",
                    }) : "Published"}
                  </span>
                </div>
                <div>
                  <span className="text-gray-400 block text-[10px] uppercase font-bold">Attribution</span>
                  <span className="font-semibold text-gray-900">
                    {activeMedia.allow_public_attribution && activeMedia.uploader_display_name
                      ? activeMedia.uploader_display_name
                      : "Verified Member"}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Public Report Concern Modal */}
      {reportingMedia && (
        <div className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-white w-full max-w-md rounded-2xl shadow-xl overflow-hidden p-6 space-y-4">
            <div className="flex justify-between items-center pb-2 border-b border-gray-100">
              <h3 className="font-bold text-gray-900 text-sm flex items-center gap-2">
                <Flag className="w-4 h-4 text-amber-600" />
                Report Media Concern
              </h3>
              <button
                type="button"
                onClick={() => setReportingMedia(null)}
                className="text-gray-400 hover:text-gray-600"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            {reportSuccess && <Alert type="success">{reportSuccess}</Alert>}
            {reportError && <Alert type="error">{reportError}</Alert>}

            {!reportSuccess && (
              <form onSubmit={handleReportSubmit} className="space-y-3 text-xs">
                <div>
                  <label className="block font-semibold text-gray-700 mb-1">Reason for Report</label>
                  <select
                    value={reportReason}
                    onChange={(e) => setReportReason(e.target.value)}
                    className="w-full rounded-lg border-gray-300 text-xs py-2 px-2.5 bg-gray-50 focus:ring-emerald-600"
                  >
                    <option value="privacy_violation">Privacy violation / Identifiable person</option>
                    <option value="missing_consent">Missing or withdrawn consent</option>
                    <option value="incorrect_event">Incorrect event attribution</option>
                    <option value="inappropriate_content">Inappropriate or offensive content</option>
                    <option value="copyright_concern">Copyright or intellectual property</option>
                    <option value="personal_safety">Personal safety concern</option>
                    <option value="other">Other issue</option>
                  </select>
                </div>

                <div>
                  <label className="block font-semibold text-gray-700 mb-1">Your Email (Optional, for updates)</label>
                  <input
                    type="email"
                    value={reportEmail}
                    onChange={(e) => setReportEmail(e.target.value)}
                    placeholder="you@example.com"
                    className="w-full rounded-lg border-gray-300 text-xs py-2 px-2.5 focus:ring-emerald-600"
                  />
                </div>

                <div>
                  <label className="block font-semibold text-gray-700 mb-1">Detailed Explanation *</label>
                  <textarea
                    required
                    rows={3}
                    value={reportDetails}
                    onChange={(e) => setReportDetails(e.target.value)}
                    placeholder="Please explain the nature of your concern..."
                    className="w-full rounded-lg border-gray-300 text-xs p-2.5 focus:ring-emerald-600"
                  />
                </div>

                <div className="pt-2 flex justify-end gap-2">
                  <button
                    type="button"
                    onClick={() => setReportingMedia(null)}
                    className="px-3 py-1.5 rounded-lg border border-gray-300 font-medium text-gray-600 hover:bg-gray-50"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    disabled={reportSubmitting}
                    className="px-4 py-1.5 rounded-lg bg-emerald-800 text-white font-bold hover:bg-emerald-900 disabled:opacity-50"
                  >
                    {reportSubmitting ? "Submitting..." : "Submit Report"}
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}

      {/* Formal Takedown Request Modal */}
      {takedownMedia && (
        <div className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-white w-full max-w-lg rounded-2xl shadow-xl overflow-hidden p-6 space-y-4">
            <div className="flex justify-between items-center pb-2 border-b border-gray-100">
              <h3 className="font-bold text-gray-900 text-sm flex items-center gap-2">
                <Shield className="w-4 h-4 text-rose-600" />
                Formal Media Takedown Request
              </h3>
              <button
                type="button"
                onClick={() => setTakedownMedia(null)}
                className="text-gray-400 hover:text-gray-600"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <p className="text-xs text-gray-500 leading-relaxed">
              If you appear in this media and wish to withdraw consent, or if this publication violates statutory privacy rules or copyright, please complete this formal notice.
            </p>

            {takedownSuccess && <Alert type="success">{takedownSuccess}</Alert>}
            {takedownError && <Alert type="error">{takedownError}</Alert>}

            {!takedownSuccess && (
              <form onSubmit={handleTakedownSubmit} className="space-y-3 text-xs">
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <div>
                    <label className="block font-semibold text-gray-700 mb-1">Your Full Name *</label>
                    <input
                      type="text"
                      required
                      value={takedownName}
                      onChange={(e) => setTakedownName(e.target.value)}
                      placeholder="Jane Doe"
                      className="w-full rounded-lg border-gray-300 text-xs py-2 px-2.5 focus:ring-emerald-600"
                    />
                  </div>
                  <div>
                    <label className="block font-semibold text-gray-700 mb-1">Your Email Address *</label>
                    <input
                      type="email"
                      required
                      value={takedownEmail}
                      onChange={(e) => setTakedownEmail(e.target.value)}
                      placeholder="jane@example.com"
                      className="w-full rounded-lg border-gray-300 text-xs py-2 px-2.5 focus:ring-emerald-600"
                    />
                  </div>
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <div>
                    <label className="block font-semibold text-gray-700 mb-1">Relationship to Media *</label>
                    <select
                      value={takedownRelation}
                      onChange={(e) => setTakedownRelation(e.target.value)}
                      className="w-full rounded-lg border-gray-300 text-xs py-2 px-2.5 bg-gray-50 focus:ring-emerald-600"
                    >
                      <option value="subject_in_media">I am pictured / depicted in the media</option>
                      <option value="uploader">I am the original uploader</option>
                      <option value="legal_representative">Legal guardian / representative</option>
                      <option value="concerned_citizen">Concerned citizen</option>
                    </select>
                  </div>
                  <div>
                    <label className="block font-semibold text-gray-700 mb-1">Legal Grounds *</label>
                    <select
                      value={takedownReason}
                      onChange={(e) => setTakedownReason(e.target.value)}
                      className="w-full rounded-lg border-gray-300 text-xs py-2 px-2.5 bg-gray-50 focus:ring-emerald-600"
                    >
                      <option value="privacy_violation">Privacy violation</option>
                      <option value="consent_withdrawn">Withdrawal of consent</option>
                      <option value="safety_concern">Personal safety or safeguarding</option>
                      <option value="copyright_infringement">Copyright infringement</option>
                      <option value="defamation">Defamation / Inaccurate portrayals</option>
                      <option value="other">Other legal grounds</option>
                    </select>
                  </div>
                </div>

                <div>
                  <label className="block font-semibold text-gray-700 mb-1">Supporting Details *</label>
                  <textarea
                    required
                    rows={3}
                    value={takedownDetails}
                    onChange={(e) => setTakedownDetails(e.target.value)}
                    placeholder="Provide specific details identifying yourself in the photo/video or explaining the grounds for removal..."
                    className="w-full rounded-lg border-gray-300 text-xs p-2.5 focus:ring-emerald-600"
                  />
                </div>

                <div className="pt-2 flex justify-end gap-2">
                  <button
                    type="button"
                    onClick={() => setTakedownMedia(null)}
                    className="px-3 py-1.5 rounded-lg border border-gray-300 font-medium text-gray-600 hover:bg-gray-50"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    disabled={takedownSubmitting}
                    className="px-4 py-1.5 rounded-lg bg-rose-700 text-white font-bold hover:bg-rose-800 disabled:opacity-50"
                  >
                    {takedownSubmitting ? "Submitting..." : "Submit Takedown Request"}
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

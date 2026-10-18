"use client";

import React, { useEffect, useState, useRef } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  Video,
  Upload,
  CheckCircle2,
  AlertCircle,
  Play,
  Pause,
  Trash2,
  RefreshCw,
  Eye,
  Sliders,
  Sparkles,
  ArrowLeft,
  Film,
  FileVideo,
  Power,
  RotateCcw,
  Image as ImageIcon,
} from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import {
  HeroMedia,
  ActiveHeroMediaResponse,
  PresignedHeroUploadResponse,
  ToggleHeroMediaResponse,
} from "@/lib/types";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";

const MAX_VIDEO_SIZE_BYTES = 50 * 1024 * 1024; // 50 MB

export default function AdminHeroMediaPage() {
  const router = useRouter();
  const { user, isCoordinator, isLoading: authLoading } = useAuth();

  const [activeMedia, setActiveMedia] = useState<HeroMedia | null>(null);
  const [mediaList, setMediaList] = useState<HeroMedia[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  // Upload state
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const [title, setTitle] = useState("");
  const [uploading, setUploading] = useState(false);
  const [uploadProgress, setUploadProgress] = useState(0);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Poster upload state
  const [posterTargetId, setPosterTargetId] = useState<string | null>(null);
  const posterInputRef = useRef<HTMLInputElement>(null);

  const isSuperOrNationalAdmin =
    user?.primary_role === "super_admin" || user?.primary_role === "national_admin";

  useEffect(() => {
    if (!authLoading && (!user || !isCoordinator)) {
      router.push("/login?redirect=/admin/hero");
      return;
    }

    if (user && isCoordinator) {
      loadMedia();
    }
  }, [user, isCoordinator, authLoading, router]);

  async function loadMedia() {
    setLoading(true);
    setError(null);
    try {
      const [activeRes, listRes] = await Promise.all([
        apiRequest<ActiveHeroMediaResponse>("/homepage/hero-media"),
        apiRequest<HeroMedia[]>("/admin/hero-media"),
      ]);

      if (activeRes?.active && activeRes.media) {
        setActiveMedia(activeRes.media);
      } else {
        setActiveMedia(null);
      }
      setMediaList(listRes || []);
    } catch (err: any) {
      setError(err.message || "Failed to load hero media configuration");
    } finally {
      setLoading(false);
    }
  }

  function handleFileSelect(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;

    setError(null);
    setSuccess(null);

    // Validate size
    if (file.size > MAX_VIDEO_SIZE_BYTES) {
      setError(
        `Selected video exceeds 50 MB (${(file.size / (1024 * 1024)).toFixed(1)} MB). Please compress the video before uploading.`
      );
      if (fileInputRef.current) fileInputRef.current.value = "";
      return;
    }

    // Validate type
    const validTypes = ["video/mp4", "video/webm"];
    const ext = file.name.substring(file.name.lastIndexOf(".")).toLowerCase();
    if (!validTypes.includes(file.type) && ext !== ".mp4" && ext !== ".webm") {
      setError("Only MP4 (H.264 recommended) and WebM video formats are supported.");
      if (fileInputRef.current) fileInputRef.current.value = "";
      return;
    }

    setSelectedFile(file);
    if (!title) {
      setTitle(file.name.replace(/\.[^/.]+$/, "").replace(/[-_]/g, " "));
    }

    // Create local object URL for instant preview
    if (previewUrl) {
      URL.revokeObjectURL(previewUrl);
    }
    const localUrl = URL.createObjectURL(file);
    setPreviewUrl(localUrl);
  }

  function clearSelection() {
    if (previewUrl) {
      URL.revokeObjectURL(previewUrl);
    }
    setSelectedFile(null);
    setPreviewUrl(null);
    setTitle("");
    setUploadProgress(0);
    if (fileInputRef.current) {
      fileInputRef.current.value = "";
    }
  }

  async function handleUploadAndSave() {
    if (!selectedFile) {
      setError("Please select a video file first.");
      return;
    }

    setUploading(true);
    setError(null);
    setSuccess(null);
    setUploadProgress(10);

    try {
      // 1. Request presigned upload authorization from Go API
      const presign = await apiRequest<PresignedHeroUploadResponse>(
        "/admin/hero-media/presign",
        {
          method: "POST",
          body: JSON.stringify({
            filename: selectedFile.name,
            content_type: selectedFile.type || "video/mp4",
            file_size_bytes: selectedFile.size,
            title: title.trim() || "Homepage Hero Background Video",
          }),
        }
      );

      setUploadProgress(30);

      // 2. Direct upload to Cloudflare R2 via presigned PUT or direct fallback
      let directPutSucceeded = false;
      if (presign.upload_url && !presign.upload_url.includes("mock-presign")) {
        try {
          const putRes = await fetch(presign.upload_url, {
            method: "PUT",
            headers: {
              "Content-Type": selectedFile.type || "video/mp4",
            },
            body: selectedFile,
          });
          if (putRes.ok) {
            directPutSucceeded = true;
          }
        } catch (_) {
          // Direct PUT failed or blocked by CORS in dev; fall through to server direct upload
        }
      }

      if (!directPutSucceeded) {
        // Fallback: direct server-mediated upload
        const formData = new FormData();
        formData.append("file", selectedFile);
        formData.append("title", title.trim() || "Homepage Hero Background Video");

        await apiRequest<HeroMedia>("/admin/hero-media/upload", {
          method: "POST",
          body: formData,
        });
      }

      setUploadProgress(100);
      setSuccess("Video uploaded successfully to Cloudflare R2! It is saved in your media library.");
      clearSelection();
      await loadMedia();
    } catch (err: any) {
      setError(err.message || "Failed to upload video to Cloudflare R2.");
    } finally {
      setUploading(false);
    }
  }

  async function handlePublish(id: string) {
    setError(null);
    setSuccess(null);
    try {
      await apiRequest<HeroMedia>(`/admin/hero-media/${id}/publish`, {
        method: "POST",
      });
      setSuccess("Hero video published! The public homepage now displays this video background.");
      await loadMedia();
    } catch (err: any) {
      setError(err.message || "Failed to publish hero video.");
    }
  }

  async function handleTogglePlayback(enabled: boolean) {
    setError(null);
    setSuccess(null);
    try {
      const res = await apiRequest<ToggleHeroMediaResponse>("/admin/hero-media/toggle", {
        method: "PATCH",
        body: JSON.stringify({ enabled }),
      });
      setSuccess(res.message);
      await loadMedia();
    } catch (err: any) {
      setError(err.message || "Failed to toggle hero video playback.");
    }
  }

  async function handleDelete(id: string) {
    if (!confirm("Are you sure you want to permanently delete this video from Cloudflare R2?")) {
      return;
    }

    setError(null);
    setSuccess(null);
    try {
      await apiRequest(`/admin/hero-media/${id}`, {
        method: "DELETE",
      });
      setSuccess("Video removed from media library and Cloudflare R2 storage.");
      await loadMedia();
    } catch (err: any) {
      setError(err.message || "Failed to delete video.");
    }
  }

  async function handlePosterFileSelect(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file || !posterTargetId) return;

    if (file.size > 5 * 1024 * 1024) {
      setError("Poster image exceeds 5 MB. Please select a smaller JPG, PNG, or WebP image.");
      return;
    }

    setError(null);
    setSuccess(null);
    try {
      const formData = new FormData();
      formData.append("file", file);

      await apiRequest<HeroMedia>(`/admin/hero-media/${posterTargetId}/poster`, {
        method: "POST",
        body: formData,
      });

      setSuccess("Poster image attached successfully!");
      setPosterTargetId(null);
      if (posterInputRef.current) posterInputRef.current.value = "";
      await loadMedia();
    } catch (err: any) {
      setError(err.message || "Failed to upload poster image.");
    }
  }

  if (authLoading || loading) {
    return <Spinner text="Loading Homepage Hero Media workspace..." />;
  }

  if (!isSuperOrNationalAdmin) {
    return (
      <div className="max-w-4xl mx-auto px-4 py-16 text-center space-y-4">
        <Alert type="error">
          Access restricted. Only National Administrators and the Super Admin are authorized to manage Homepage Hero Media.
        </Alert>
        <Link
          href="/admin"
          className="inline-flex items-center gap-2 px-4 py-2 bg-emerald-900 text-white rounded-xl text-xs font-bold"
        >
          <ArrowLeft className="w-4 h-4" />
          Back to Admin Workspace
        </Link>
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-8">
      {/* Hidden file input for poster upload */}
      <input
        type="file"
        ref={posterInputRef}
        onChange={handlePosterFileSelect}
        accept="image/jpeg,image/png,image/webp"
        className="hidden"
      />

      {/* Header & Breadcrumb */}
      <div className="space-y-2">
        <div className="flex items-center gap-2 text-xs text-gray-500 font-medium">
          <Link href="/admin" className="hover:text-emerald-900 transition flex items-center gap-1">
            <ArrowLeft className="w-3.5 h-3.5" />
            Administration
          </Link>
          <span>/</span>
          <span className="text-gray-900 font-bold">Homepage Hero Media</span>
        </div>

        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div>
            <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900 flex items-center gap-2.5">
              <Film className="w-7 h-7 text-emerald-800" />
              Homepage Hero Background Video
            </h1>
            <p className="text-xs sm:text-sm text-gray-500">
              Manage dynamic HTML5 video playback for the public Ratels homepage hero section.
            </p>
          </div>

          <div className="flex items-center gap-2">
            <Link
              href="/"
              target="_blank"
              className="inline-flex items-center gap-1.5 px-3.5 py-2 text-xs font-bold text-gray-700 bg-white border border-gray-300 rounded-xl hover:bg-gray-50 transition shadow-sm"
            >
              <Eye className="w-4 h-4" />
              View Public Homepage
            </Link>
            <button
              onClick={loadMedia}
              className="inline-flex items-center gap-1.5 px-3 py-2 text-xs font-semibold text-gray-600 bg-white border border-gray-200 rounded-xl hover:bg-gray-50 transition"
              title="Refresh Media"
            >
              <RefreshCw className="w-4 h-4" />
            </button>
          </div>
        </div>
      </div>

      {error && <Alert type="error">{error}</Alert>}
      {success && <Alert type="success">{success}</Alert>}

      {/* Live Status & Current Active Video Banner */}
      <div className="bg-white rounded-2xl border border-gray-200 p-6 shadow-sm space-y-6">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-gray-100 pb-4">
          <div className="space-y-1">
            <div className="flex items-center gap-2">
              <span className="text-xs font-bold uppercase tracking-wider text-gray-500">
                Current Hero Background Status
              </span>
              {activeMedia ? (
                <Badge variant="success" size="sm">
                  ● Live Dynamic Video
                </Badge>
              ) : (
                <Badge variant="neutral" size="sm">
                  Default Green Gradient Active
                </Badge>
              )}
            </div>
            <h2 className="text-lg font-bold text-gray-900">
              {activeMedia ? activeMedia.title : "Default Ratels Movement Green Gradient"}
            </h2>
          </div>

          {/* Quick Actions */}
          <div className="flex flex-wrap items-center gap-2">
            {activeMedia ? (
              <button
                type="button"
                onClick={() => handleTogglePlayback(false)}
                className="inline-flex items-center gap-1.5 px-4 py-2 bg-amber-50 text-amber-900 border border-amber-300 hover:bg-amber-100 rounded-xl text-xs font-bold transition shadow-sm"
              >
                <Power className="w-3.5 h-3.5 text-amber-700" />
                Disable Video Playback (Show Gradient)
              </button>
            ) : mediaList.length > 0 ? (
              <button
                type="button"
                onClick={() => handleTogglePlayback(true)}
                className="inline-flex items-center gap-1.5 px-4 py-2 bg-emerald-900 text-white hover:bg-emerald-950 rounded-xl text-xs font-bold transition shadow-sm"
              >
                <Power className="w-3.5 h-3.5 text-emerald-300" />
                Re-enable Video Playback
              </button>
            ) : null}
          </div>
        </div>

        {/* Live Hero Video Preview */}
        {activeMedia ? (
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
            <div className="lg:col-span-2 aspect-video bg-gray-900 rounded-xl overflow-hidden relative border border-gray-800 shadow-inner">
              <video
                src={activeMedia.video_url}
                poster={activeMedia.poster_url || undefined}
                autoPlay
                muted
                loop
                playsInline
                controls
                className="w-full h-full object-cover"
              />
            </div>
            <div className="space-y-4 text-xs">
              <div className="p-4 bg-gray-50 rounded-xl border border-gray-200 space-y-2">
                <div className="font-bold text-gray-800 text-sm">Media Properties</div>
                <div className="flex justify-between py-1 border-b border-gray-200/60">
                  <span className="text-gray-500">Cloudflare R2 Key:</span>
                  <span className="font-mono text-gray-700 truncate max-w-[180px]">{activeMedia.video_key}</span>
                </div>
                <div className="flex justify-between py-1 border-b border-gray-200/60">
                  <span className="text-gray-500">File Size:</span>
                  <span className="font-bold text-gray-800">
                    {(activeMedia.file_size_bytes / (1024 * 1024)).toFixed(2)} MB
                  </span>
                </div>
                <div className="flex justify-between py-1 border-b border-gray-200/60">
                  <span className="text-gray-500">Content Type:</span>
                  <span className="font-mono text-gray-800">{activeMedia.content_type}</span>
                </div>
                <div className="flex justify-between py-1 border-b border-gray-200/60">
                  <span className="text-gray-500">Published At:</span>
                  <span className="text-gray-700">
                    {activeMedia.published_at ? new Date(activeMedia.published_at).toLocaleString() : "Active"}
                  </span>
                </div>
                {activeMedia.poster_url ? (
                  <div className="flex justify-between py-1">
                    <span className="text-gray-500">Poster Attached:</span>
                    <span className="text-emerald-700 font-bold">Yes (Custom)</span>
                  </div>
                ) : (
                  <div className="pt-2">
                    <button
                      type="button"
                      onClick={() => {
                        setPosterTargetId(activeMedia.id);
                        posterInputRef.current?.click();
                      }}
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-white border border-gray-300 rounded-lg text-[11px] font-semibold text-gray-700 hover:bg-gray-50"
                    >
                      <ImageIcon className="w-3.5 h-3.5 text-gray-500" />
                      Attach Poster Image (Optional)
                    </button>
                  </div>
                )}
              </div>

              <div className="p-4 bg-emerald-50 rounded-xl border border-emerald-200 text-emerald-950 space-y-1">
                <div className="font-bold flex items-center gap-1.5">
                  <CheckCircle2 className="w-4 h-4 text-emerald-700" />
                  Live in Production
                </div>
                <p className="text-[11px] text-emerald-800 leading-relaxed">
                  Served directly to public visitors through Cloudflare R2 edge caching with HTML5 autoplay, muted playback, and dark contrast overlay.
                </p>
              </div>
            </div>
          </div>
        ) : (
          <div className="p-8 bg-gradient-to-r from-emerald-950 via-emerald-900 to-gray-900 rounded-xl text-white text-center space-y-3">
            <div className="w-12 h-12 rounded-full bg-emerald-800/80 border border-emerald-700 flex items-center justify-center mx-auto text-emerald-300">
              <Sparkles className="w-6 h-6" />
            </div>
            <h3 className="text-lg font-bold">Default Green Gradient Hero is Active</h3>
            <p className="text-xs text-emerald-200 max-w-md mx-auto">
              No video is currently published, or video playback has been disabled. The public homepage displays the classic dark green gradient background.
            </p>
          </div>
        )}
      </div>

      {/* Upload New Video Section */}
      <div className="bg-white rounded-2xl border border-gray-200 p-6 shadow-sm space-y-6">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <Upload className="w-5 h-5 text-emerald-800" />
            <h2 className="text-lg font-bold text-gray-900">Upload New Hero Video</h2>
          </div>
          <p className="text-xs text-gray-500">
            Select an MP4 or WebM video file (max 50 MB) from your computer or mobile device to upload directly to Cloudflare R2.
          </p>
        </div>

        {/* Drop / Select Zone */}
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          <div className="space-y-4">
            <input
              type="file"
              ref={fileInputRef}
              onChange={handleFileSelect}
              accept="video/mp4,video/webm"
              className="hidden"
              id="hero-video-picker"
            />

            {!selectedFile ? (
              <label
                htmlFor="hero-video-picker"
                className="flex flex-col items-center justify-center border-2 border-dashed border-gray-300 hover:border-emerald-600 rounded-2xl p-8 cursor-pointer transition bg-gray-50/50 hover:bg-emerald-50/30 text-center space-y-3 group"
              >
                <div className="w-14 h-14 rounded-2xl bg-white shadow-sm border border-gray-200 flex items-center justify-center text-emerald-800 group-hover:scale-105 transition">
                  <FileVideo className="w-7 h-7" />
                </div>
                <div>
                  <span className="text-xs font-bold text-emerald-900 hover:underline">
                    Click to browse video file
                  </span>
                  <p className="text-[11px] text-gray-400 mt-1">MP4 (H.264) or WebM • Maximum 50 MB</p>
                </div>
              </label>
            ) : (
              <div className="p-4 bg-gray-50 rounded-xl border border-gray-200 space-y-3">
                <div className="flex items-start justify-between">
                  <div className="flex items-center gap-3">
                    <div className="w-10 h-10 rounded-lg bg-emerald-100 text-emerald-900 flex items-center justify-center font-bold">
                      <FileVideo className="w-5 h-5" />
                    </div>
                    <div>
                      <div className="text-xs font-bold text-gray-900 truncate max-w-[220px]">
                        {selectedFile.name}
                      </div>
                      <div className="text-[10px] text-gray-500">
                        {(selectedFile.size / (1024 * 1024)).toFixed(2)} MB • {selectedFile.type || "video/mp4"}
                      </div>
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={clearSelection}
                    className="text-gray-400 hover:text-red-600 p-1 rounded-lg transition"
                    title="Remove selected file"
                  >
                    ✕
                  </button>
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-bold text-gray-700">Video Title / Campaign Label</label>
                  <input
                    type="text"
                    value={title}
                    onChange={(e) => setTitle(e.target.value)}
                    placeholder="e.g. Movement Environmental Sanitation 2026"
                    className="w-full text-xs px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-emerald-700"
                  />
                </div>

                {uploading && (
                  <div className="space-y-1.5 pt-2">
                    <div className="flex justify-between text-[11px] font-semibold text-emerald-900">
                      <span>Uploading to Cloudflare R2...</span>
                      <span>{uploadProgress}%</span>
                    </div>
                    <div className="w-full bg-gray-200 rounded-full h-2 overflow-hidden">
                      <div
                        className="bg-emerald-700 h-2 rounded-full transition-all duration-300"
                        style={{ width: `${uploadProgress}%` }}
                      />
                    </div>
                  </div>
                )}

                <div className="pt-2 flex gap-2">
                  <button
                    type="button"
                    disabled={uploading}
                    onClick={handleUploadAndSave}
                    className="flex-1 inline-flex items-center justify-center gap-2 px-4 py-2.5 bg-emerald-900 hover:bg-emerald-950 text-white rounded-xl text-xs font-bold transition shadow-sm disabled:opacity-50"
                  >
                    <Upload className="w-4 h-4" />
                    {uploading ? "Uploading Video..." : "Upload to Cloudflare R2"}
                  </button>
                  <button
                    type="button"
                    disabled={uploading}
                    onClick={clearSelection}
                    className="px-3 py-2 border border-gray-300 rounded-xl text-xs font-semibold text-gray-600 hover:bg-gray-100 transition"
                  >
                    Cancel
                  </button>
                </div>
              </div>
            )}
          </div>

          {/* Local Preview Player */}
          <div className="space-y-2">
            <span className="text-xs font-bold text-gray-700">Local Preview (Before Upload)</span>
            <div className="aspect-video bg-gray-900 rounded-xl overflow-hidden relative border border-gray-200 flex items-center justify-center">
              {previewUrl ? (
                <video
                  src={previewUrl}
                  controls
                  className="w-full h-full object-cover"
                />
              ) : (
                <div className="text-center text-gray-400 p-6 space-y-2">
                  <Play className="w-8 h-8 mx-auto opacity-40" />
                  <p className="text-xs">Select a video file to preview playback and check cropping.</p>
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Compression & Best Practice Tips */}
        <div className="p-4 bg-emerald-50/60 rounded-xl border border-emerald-100 text-xs text-emerald-950 space-y-1.5">
          <div className="font-bold flex items-center gap-1.5 text-emerald-900">
            <Sliders className="w-4 h-4 text-emerald-700" />
            Recommended Video Compression Settings
          </div>
          <p className="text-[11px] text-emerald-800 leading-relaxed">
            • <strong>Codec:</strong> H.264 (AVC) in MP4 container for universal cross-browser and mobile support. <br />
            • <strong>Resolution:</strong> 1080p (1920×1080) or 720p (1280×720) with 16:9 aspect ratio. <br />
            • <strong>Bitrate:</strong> 2–4 Mbps (keeps 15–30 second background loops between 5 MB to 15 MB for instantaneous mobile loading). <br />
            • <strong>Framerate:</strong> 24–30 FPS with muted audio track.
          </p>
        </div>
      </div>

      {/* Media Library / Uploaded Videos Table */}
      <div className="bg-white rounded-2xl border border-gray-200 p-6 shadow-sm space-y-4">
        <div className="flex items-center justify-between">
          <div className="space-y-1">
            <h2 className="text-lg font-bold text-gray-900 flex items-center gap-2">
              <Video className="w-5 h-5 text-emerald-800" />
              Hero Media Library ({mediaList.length})
            </h2>
            <p className="text-xs text-gray-500">
              All videos stored in Cloudflare R2 bucket (`ratels`). Click &quot;Publish as Hero&quot; to atomically activate any video.
            </p>
          </div>
        </div>

        {mediaList.length === 0 ? (
          <div className="text-center py-12 border border-dashed border-gray-200 rounded-xl space-y-2">
            <FileVideo className="w-10 h-10 text-gray-400 mx-auto" />
            <h3 className="text-sm font-bold text-gray-800">No Videos Uploaded Yet</h3>
            <p className="text-xs text-gray-500">
              Upload your first video above to activate dynamic homepage hero playback.
            </p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead className="bg-gray-50 text-gray-600 font-semibold uppercase tracking-wider border-b border-gray-200">
                <tr>
                  <th className="py-3 px-4">Preview</th>
                  <th className="py-3 px-4">Title / Cloudflare Key</th>
                  <th className="py-3 px-4">Size & Format</th>
                  <th className="py-3 px-4">Status</th>
                  <th className="py-3 px-4">Created Date</th>
                  <th className="py-3 px-4 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {mediaList.map((item) => (
                  <tr key={item.id} className="hover:bg-gray-50/70 transition">
                    <td className="py-3 px-4">
                      <div className="w-24 aspect-video bg-gray-900 rounded-lg overflow-hidden border border-gray-200 relative group">
                        <video
                          src={item.video_url}
                          poster={item.poster_url || undefined}
                          muted
                          playsInline
                          preload="metadata"
                          className="w-full h-full object-cover"
                        />
                      </div>
                    </td>
                    <td className="py-3 px-4 space-y-0.5">
                      <div className="font-bold text-gray-900">{item.title}</div>
                      <div className="font-mono text-[10px] text-gray-500 truncate max-w-[200px]" title={item.video_key}>
                        {item.video_key}
                      </div>
                    </td>
                    <td className="py-3 px-4 space-y-0.5">
                      <div className="font-bold text-gray-800">
                        {(item.file_size_bytes / (1024 * 1024)).toFixed(2)} MB
                      </div>
                      <div className="text-[10px] text-gray-400">{item.content_type}</div>
                    </td>
                    <td className="py-3 px-4">
                      {item.is_active ? (
                        <Badge variant="success" size="sm">
                          ● Published & Active
                        </Badge>
                      ) : item.published_at ? (
                        <Badge variant="neutral" size="sm">
                          Previously Published
                        </Badge>
                      ) : (
                        <Badge variant="warning" size="sm">
                          Draft / Inactive
                        </Badge>
                      )}
                    </td>
                    <td className="py-3 px-4 text-gray-500 text-[11px]">
                      {new Date(item.created_at).toLocaleDateString()}
                    </td>
                    <td className="py-3 px-4 text-right space-x-2">
                      {!item.is_active ? (
                        <button
                          type="button"
                          onClick={() => handlePublish(item.id)}
                          className="inline-flex items-center gap-1 px-3 py-1.5 bg-emerald-900 hover:bg-emerald-950 text-white rounded-lg font-bold text-[11px] transition shadow-sm"
                        >
                          <Play className="w-3 h-3" />
                          Publish as Hero
                        </button>
                      ) : (
                        <button
                          type="button"
                          onClick={() => handleTogglePlayback(false)}
                          className="inline-flex items-center gap-1 px-3 py-1.5 bg-amber-50 hover:bg-amber-100 text-amber-900 border border-amber-300 rounded-lg font-semibold text-[11px] transition"
                        >
                          <Pause className="w-3 h-3 text-amber-700" />
                          Deactivate
                        </button>
                      )}

                      <button
                        type="button"
                        onClick={() => {
                          setPosterTargetId(item.id);
                          posterInputRef.current?.click();
                        }}
                        className="inline-flex items-center gap-1 px-2.5 py-1.5 bg-white border border-gray-300 rounded-lg text-gray-700 hover:bg-gray-50 text-[11px] transition"
                        title="Upload/Update Poster Image"
                      >
                        <ImageIcon className="w-3 h-3 text-gray-500" />
                        Poster
                      </button>

                      <button
                        type="button"
                        onClick={() => handleDelete(item.id)}
                        className="inline-flex items-center p-1.5 text-gray-400 hover:text-red-700 hover:bg-red-50 rounded-lg transition"
                        title="Delete Video"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}

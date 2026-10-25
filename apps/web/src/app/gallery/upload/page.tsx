"use client";

import React, { useState, useEffect, useRef, Suspense } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import {
  Upload,
  Camera,
  Video,
  Calendar,
  CheckCircle2,
  AlertTriangle,
  ArrowLeft,
  Shield,
  FileCheck,
  Eye,
  X,
  Info,
} from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import { EventInstance } from "@/lib/types";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";
import { Badge } from "@/components/ui/Badge";

function GalleryUploadForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const preselectedEventId = searchParams.get("event_id");

  const { user, isLoading } = useAuth();

  // Events list for association
  const [events, setEvents] = useState<EventInstance[]>([]);
  const [loadingEvents, setLoadingEvents] = useState<boolean>(true);

  // Form State
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const [mediaType, setMediaType] = useState<"image" | "video">("image");
  const [eventId, setEventId] = useState<string>(preselectedEventId || "");
  const [caption, setCaption] = useState<string>("");
  const [description, setDescription] = useState<string>("");
  const [allowAttribution, setAllowAttribution] = useState<boolean>(true);

  // Mandatory Consent Declarations
  const [hasOwnerPermission, setHasOwnerPermission] = useState<boolean>(false);
  const [understandsPublicDisplay, setUnderstandsPublicDisplay] = useState<boolean>(false);
  const [hasIdentifiableConsent, setHasIdentifiableConsent] = useState<boolean>(false);
  const [hasNoConfidentialInfo, setHasNoConfidentialInfo] = useState<boolean>(false);
  const [hasMinors, setHasMinors] = useState<boolean>(false);
  const [hasGuardianConsent, setHasGuardianConsent] = useState<boolean>(false);
  const [consentNotes, setConsentNotes] = useState<string>("");

  // Upload Progress & Feedback
  const [uploading, setUploading] = useState<boolean>(false);
  const [uploadProgress, setUploadProgress] = useState<number>(0);
  const [statusMessage, setStatusMessage] = useState<string>("");
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const fileInputRef = useRef<HTMLInputElement>(null);

  // Require login
  useEffect(() => {
    if (!isLoading && !user) {
      router.push("/login?redirect=" + encodeURIComponent("/gallery/upload"));
    }
  }, [user, isLoading, router]);

  // Load available published cleanup events
  useEffect(() => {
    async function loadEvents() {
      try {
        const data = await apiRequest<EventInstance[]>("/activities/events");
        setEvents(data || []);
        if (!eventId && data && data.length > 0) {
          if (preselectedEventId && data.some((e) => e.id === preselectedEventId)) {
            setEventId(preselectedEventId);
          } else {
            setEventId(data[0].id);
          }
        }
      } catch (e: any) {
        console.error("Failed to load events for media upload", e);
      } finally {
        setLoadingEvents(false);
      }
    }
    loadEvents();
  }, [preselectedEventId]);

  // Handle file selection
  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files[0]) {
      processSelectedFile(e.target.files[0]);
    }
  };

  const processSelectedFile = (file: File) => {
    setError(null);

    // Validate type
    const isImage = file.type.startsWith("image/");
    const isVideo = file.type.startsWith("video/");

    if (!isImage && !isVideo) {
      setError("Please select a supported image (JPG, PNG, WebP) or video (MP4, WebM) file.");
      return;
    }

    if (isImage && file.size > 10 * 1024 * 1024) {
      setError("Photographs must be 10 MB or smaller.");
      return;
    }

    if (isVideo && file.size > 100 * 1024 * 1024) {
      setError("Videos must be 100 MB or smaller.");
      return;
    }

    setSelectedFile(file);
    setMediaType(isVideo ? "video" : "image");

    // Create local object URL for preview
    const url = URL.createObjectURL(file);
    setPreviewUrl(url);
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    if (e.dataTransfer.files && e.dataTransfer.files[0]) {
      processSelectedFile(e.dataTransfer.files[0]);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSuccess(null);

    if (!selectedFile) {
      setError("Please select a media file to upload.");
      return;
    }

    if (!eventId) {
      setError("Please choose the cleanup event associated with this media.");
      return;
    }

    if (!caption.trim()) {
      setError("Please provide a descriptive caption for this media.");
      return;
    }

    // Consent Validation
    if (!hasOwnerPermission || !understandsPublicDisplay || !hasIdentifiableConsent || !hasNoConfidentialInfo) {
      setError("All mandatory participant and privacy consent declaration checkboxes must be confirmed.");
      return;
    }

    if (hasMinors && !hasGuardianConsent) {
      setError("Guardian consent is strictly mandatory when minors under 18 appear in the photograph or video.");
      return;
    }

    setUploading(true);
    setUploadProgress(10);
    setStatusMessage("Requesting secure upload authorization...");

    try {
      // Step 1: Request presigned quarantine upload key from backend
      let storageKey = "";
      let presignSuccessful = false;

      try {
        const presignRes = await apiRequest<{
          upload_url: string;
          storage_key: string;
          expires_in_seconds: number;
        }>("/me/gallery/presign", {
          method: "POST",
          body: JSON.stringify({
            event_id: eventId,
            media_type: mediaType,
            filename: selectedFile.name,
            content_type: selectedFile.type,
            file_size_bytes: selectedFile.size,
          }),
        });

        storageKey = presignRes.storage_key;
        setUploadProgress(30);
        setStatusMessage("Uploading media to private quarantine storage...");

        // Upload file bytes to presigned URL
        const uploadRes = await fetch(presignRes.upload_url, {
          method: "PUT",
          headers: {
            "Content-Type": selectedFile.type,
          },
          body: selectedFile,
        });

        if (uploadRes.ok) {
          presignSuccessful = true;
          setUploadProgress(70);
          setStatusMessage("Recording submission metadata and consent declaration...");

          // Submit media metadata referencing quarantine storage key
          await apiRequest("/me/gallery/submit", {
            method: "POST",
            body: JSON.stringify({
              event_id: eventId,
              media_type: mediaType,
              caption: caption.trim(),
              description: description.trim() || undefined,
              storage_key: storageKey,
              mime_type: selectedFile.type,
              file_size_bytes: selectedFile.size,
              allow_public_attribution: allowAttribution,
              consent: {
                has_owner_permission: hasOwnerPermission,
                understands_public_display: understandsPublicDisplay,
                has_identifiable_consent: hasIdentifiableConsent,
                has_no_confidential_info: hasNoConfidentialInfo,
                has_minors: hasMinors,
                has_guardian_consent: hasGuardianConsent,
                consent_notes: consentNotes.trim() || undefined,
              },
            }),
          });
        }
      } catch (presignErr) {
        console.warn("Presigned upload did not succeed, attempting direct upload fallback...", presignErr);
      }

      // Step 2: Fallback to direct multipart upload if presigned flow was unavailable
      if (!presignSuccessful) {
        setStatusMessage("Uploading via direct multipart secure pipeline...");
        setUploadProgress(40);

        const formData = new FormData();
        formData.append("file", selectedFile);
        formData.append("event_id", eventId);
        formData.append("media_type", mediaType);
        formData.append("caption", caption.trim());
        if (description.trim()) formData.append("description", description.trim());
        formData.append("allow_public_attribution", allowAttribution ? "true" : "false");
        formData.append("has_owner_permission", hasOwnerPermission ? "true" : "false");
        formData.append("understands_public_display", understandsPublicDisplay ? "true" : "false");
        formData.append("has_identifiable_consent", hasIdentifiableConsent ? "true" : "false");
        formData.append("has_no_confidential_info", hasNoConfidentialInfo ? "true" : "false");
        formData.append("has_minors", hasMinors ? "true" : "false");
        formData.append("has_guardian_consent", hasGuardianConsent ? "true" : "false");
        if (consentNotes.trim()) formData.append("consent_notes", consentNotes.trim());

        await apiRequest("/me/gallery/upload", {
          method: "POST",
          body: formData,
        });
      }

      setUploadProgress(100);
      setStatusMessage("Upload complete!");
      setSuccess("Your event media has been securely submitted! It is now pending administrative moderation and metadata sanitization before publication.");

      setTimeout(() => {
        router.push("/gallery/my-submissions");
      }, 2500);
    } catch (err: any) {
      setError(err.message || "Failed to submit event media. Please try again.");
    } finally {
      setUploading(false);
    }
  };

  if (isLoading || loadingEvents) return <Spinner text="Preparing upload interface..." />;
  if (!user) return null;

  return (
    <div className="min-h-screen bg-gray-50 py-10">
      <div className="max-w-3xl mx-auto px-4 sm:px-6 lg:px-8 space-y-6">
        {/* Back navigation */}
        <Link
          href="/gallery"
          className="inline-flex items-center gap-1.5 text-xs font-semibold text-emerald-800 hover:text-emerald-950 transition"
        >
          <ArrowLeft className="w-4 h-4" />
          <span>Return to Media Gallery</span>
        </Link>

        {/* Page Header Card */}
        <div className="bg-white rounded-2xl border border-gray-200 p-6 sm:p-8 shadow-sm space-y-3">
          <div className="flex items-center gap-2">
            <span className="p-2 rounded-lg bg-emerald-100 text-emerald-800">
              <Upload className="w-5 h-5" />
            </span>
            <div>
              <h1 className="text-xl sm:text-2xl font-black text-gray-900 tracking-tight">
                Submit Cleanup Media
              </h1>
              <p className="text-xs text-gray-500">
                Authorized member photo & video contribution for Ratels Worldwide cleanup operations
              </p>
            </div>
          </div>

          <div className="bg-emerald-50 border border-emerald-200 rounded-xl p-3.5 text-xs text-emerald-900 flex items-start gap-2.5">
            <Shield className="w-4 h-4 text-emerald-700 flex-shrink-0 mt-0.5" />
            <div>
              <span className="font-bold">Privacy & Sanitization Safeguards: </span>
              All submissions remain strictly private in quarantine until verified and approved by authorized coordinators. Prior to public gallery publication, images undergo automated 100% EXIF and GPS coordinate stripping to safeguard volunteer confidentiality.
            </div>
          </div>
        </div>

        {error && <Alert type="error">{error}</Alert>}
        {success && <Alert type="success">{success}</Alert>}

        {/* Upload Form */}
        <form onSubmit={handleSubmit} className="bg-white rounded-2xl border border-gray-200 p-6 sm:p-8 shadow-sm space-y-6">
          {/* File Picker & Drag-and-Drop Dropzone */}
          <div className="space-y-2">
            <label className="block text-xs font-bold text-gray-900 uppercase tracking-wider">
              1. Media File (Photo or Video) *
            </label>

            <div
              onDragOver={handleDragOver}
              onDrop={handleDrop}
              onClick={() => fileInputRef.current?.click()}
              className={`border-2 border-dashed rounded-2xl p-6 text-center cursor-pointer transition flex flex-col items-center justify-center min-h-[220px] ${
                selectedFile
                  ? "border-emerald-600 bg-emerald-50/30"
                  : "border-gray-300 hover:border-emerald-500 hover:bg-gray-50"
              }`}
            >
              <input
                ref={fileInputRef}
                type="file"
                accept="image/jpeg,image/png,image/webp,video/mp4,video/webm"
                onChange={handleFileChange}
                className="hidden"
              />

              {previewUrl ? (
                <div className="space-y-3 w-full max-w-sm">
                  <div className="relative aspect-video rounded-xl overflow-hidden bg-black flex items-center justify-center mx-auto shadow-sm">
                    {mediaType === "video" ? (
                      <video src={previewUrl} controls className="w-full h-full object-contain" />
                    ) : (
                      <img src={previewUrl} alt="Preview" className="w-full h-full object-contain" />
                    )}
                  </div>
                  <div className="text-xs text-gray-700 font-semibold truncate">
                    {selectedFile?.name} ({(selectedFile!.size / (1024 * 1024)).toFixed(2)} MB)
                  </div>
                  <span className="text-[11px] text-emerald-800 font-bold underline">
                    Click or drag another file to replace
                  </span>
                </div>
              ) : (
                <div className="space-y-2">
                  <div className="w-12 h-12 rounded-full bg-emerald-50 text-emerald-800 flex items-center justify-center mx-auto">
                    <Camera className="w-6 h-6" />
                  </div>
                  <div className="text-sm font-bold text-gray-900">
                    Drag and drop your cleanup photograph or video here
                  </div>
                  <p className="text-xs text-gray-500">
                    Supports JPG, PNG, WebP (max 10 MB) or MP4, WebM (max 100 MB)
                  </p>
                  <button
                    type="button"
                    className="mt-2 px-4 py-1.5 bg-white border border-gray-300 rounded-lg text-xs font-semibold text-gray-700 hover:bg-gray-50 shadow-sm"
                  >
                    Browse Files
                  </button>
                </div>
              )}
            </div>
          </div>

          {/* Event Association */}
          <div className="space-y-2">
            <label className="block text-xs font-bold text-gray-900 uppercase tracking-wider">
              2. Cleanup Event Association *
            </label>
            <select
              required
              value={eventId}
              onChange={(e) => setEventId(e.target.value)}
              className="w-full rounded-xl border-gray-300 text-sm py-2.5 px-3 bg-gray-50 focus:ring-emerald-600 focus:border-emerald-600 font-medium"
            >
              <option value="">-- Choose verified cleanup event --</option>
              {events.map((ev) => (
                <option key={ev.id} value={ev.id}>
                  {ev.title} ({ev.state_name || "State"} • {new Date(ev.start_time).toLocaleDateString("en-GB")})
                </option>
              ))}
            </select>
            <p className="text-[11px] text-gray-500">
              Media is server-validated against this cleanup event and automatically adopts its official State and LGA jurisdiction.
            </p>
          </div>

          {/* Caption & Description */}
          <div className="space-y-4">
            <div>
              <label className="block text-xs font-bold text-gray-900 uppercase tracking-wider mb-1">
                3. Media Caption *
              </label>
              <input
                type="text"
                required
                maxLength={255}
                value={caption}
                onChange={(e) => setCaption(e.target.value)}
                placeholder="e.g. Volunteers sweeping Bar Beach shoreline and segregating plastics"
                className="w-full rounded-xl border-gray-300 text-sm py-2.5 px-3 focus:ring-emerald-600 focus:border-emerald-600"
              />
            </div>

            <div>
              <label className="block text-xs font-bold text-gray-900 uppercase tracking-wider mb-1">
                4. Description / Context (Optional)
              </label>
              <textarea
                rows={3}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Provide additional details regarding the cleanup location, volunteer turn-out, or recyclable waste collection results..."
                className="w-full rounded-xl border-gray-300 text-sm p-3 focus:ring-emerald-600 focus:border-emerald-600"
              />
            </div>

            <div className="flex items-center gap-2 pt-1">
              <input
                type="checkbox"
                id="allowAttribution"
                checked={allowAttribution}
                onChange={(e) => setAllowAttribution(e.target.checked)}
                className="rounded border-gray-300 text-emerald-800 focus:ring-emerald-600 h-4 w-4"
              />
              <label htmlFor="allowAttribution" className="text-xs text-gray-700 font-medium cursor-pointer">
                Display my public member name as contributor attribution upon approval
              </label>
            </div>
          </div>

          {/* Mandatory Privacy & Safeguarding Consent Declaration */}
          <div className="pt-4 border-t border-gray-200 space-y-3">
            <div className="flex items-center gap-2">
              <FileCheck className="w-4 h-4 text-emerald-800" />
              <h3 className="text-xs font-bold text-gray-900 uppercase tracking-wider">
                5. Mandatory Consent & Privacy Declarations *
              </h3>
            </div>

            <div className="bg-gray-50 p-4 rounded-xl border border-gray-200 space-y-3 text-xs text-gray-700">
              <label className="flex items-start gap-2.5 cursor-pointer">
                <input
                  type="checkbox"
                  required
                  checked={hasOwnerPermission}
                  onChange={(e) => setHasOwnerPermission(e.target.checked)}
                  className="rounded border-gray-300 text-emerald-800 focus:ring-emerald-600 h-4 w-4 mt-0.5"
                />
                <span>
                  <strong>Owner Permission:</strong> I took this photograph/video myself or have full written permission from the copyright owner to submit it.
                </span>
              </label>

              <label className="flex items-start gap-2.5 cursor-pointer">
                <input
                  type="checkbox"
                  required
                  checked={understandsPublicDisplay}
                  onChange={(e) => setUnderstandsPublicDisplay(e.target.checked)}
                  className="rounded border-gray-300 text-emerald-800 focus:ring-emerald-600 h-4 w-4 mt-0.5"
                />
                <span>
                  <strong>Public Gallery Understanding:</strong> I understand that if approved by coordinators, this media will be displayed publicly in the Ratels Worldwide Movement Gallery.
                </span>
              </label>

              <label className="flex items-start gap-2.5 cursor-pointer">
                <input
                  type="checkbox"
                  required
                  checked={hasIdentifiableConsent}
                  onChange={(e) => setHasIdentifiableConsent(e.target.checked)}
                  className="rounded border-gray-300 text-emerald-800 focus:ring-emerald-600 h-4 w-4 mt-0.5"
                />
                <span>
                  <strong>Identifiable Subjects Consent:</strong> All identifiable volunteers or community members pictured have consented to being photographed and publicly displayed for Ratels activities.
                </span>
              </label>

              <label className="flex items-start gap-2.5 cursor-pointer">
                <input
                  type="checkbox"
                  required
                  checked={hasNoConfidentialInfo}
                  onChange={(e) => setHasNoConfidentialInfo(e.target.checked)}
                  className="rounded border-gray-300 text-emerald-800 focus:ring-emerald-600 h-4 w-4 mt-0.5"
                />
                <span>
                  <strong>No Confidential Info:</strong> This media does not disclose confidential identity numbers, private residence details, vehicle license plates, or defamatory scenes.
                </span>
              </label>

              {/* Minors Safeguard Switch */}
              <div className="pt-2 border-t border-gray-200 space-y-2">
                <label className="flex items-center gap-2.5 font-bold text-gray-900 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={hasMinors}
                    onChange={(e) => {
                      setHasMinors(e.target.checked);
                      if (!e.target.checked) setHasGuardianConsent(false);
                    }}
                    className="rounded border-gray-300 text-emerald-800 focus:ring-emerald-600 h-4 w-4"
                  />
                  <span>Are any minors (children under 18 years) pictured in this media?</span>
                </label>

                {hasMinors && (
                  <div className="bg-amber-50 border border-amber-200 p-3 rounded-lg space-y-2">
                    <div className="flex items-start gap-2 text-amber-900 font-semibold">
                      <AlertTriangle className="w-4 h-4 text-amber-600 flex-shrink-0 mt-0.5" />
                      <span>Special Safeguarding Policy for Minors:</span>
                    </div>
                    <label className="flex items-start gap-2 cursor-pointer text-amber-950 font-medium">
                      <input
                        type="checkbox"
                        required={hasMinors}
                        checked={hasGuardianConsent}
                        onChange={(e) => setHasGuardianConsent(e.target.checked)}
                        className="rounded border-amber-400 text-emerald-800 focus:ring-emerald-600 h-4 w-4 mt-0.5"
                      />
                      <span>
                        I explicitly confirm that verified parental or legal guardian consent was obtained for every child appearing in this media.
                      </span>
                    </label>
                  </div>
                )}
              </div>

              <div>
                <label className="block text-[11px] text-gray-500 mb-1">
                  Consent Notes or Safeguarding Disclosures (Optional)
                </label>
                <input
                  type="text"
                  value={consentNotes}
                  onChange={(e) => setConsentNotes(e.target.value)}
                  placeholder="e.g. Verbal consent obtained from group volunteers at Bar Beach assembly point"
                  className="w-full rounded-lg border-gray-300 text-xs py-1.5 px-2.5 focus:ring-emerald-600"
                />
              </div>
            </div>
          </div>

          {/* Upload Progress Bar */}
          {uploading && (
            <div className="space-y-2">
              <div className="flex justify-between text-xs font-semibold text-emerald-900">
                <span>{statusMessage}</span>
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

          {/* Submit Action */}
          <div className="pt-4 flex items-center justify-end gap-3">
            <Link
              href="/gallery"
              className="px-4 py-2.5 rounded-xl border border-gray-300 text-xs font-semibold text-gray-700 hover:bg-gray-50 transition"
            >
              Cancel
            </Link>
            <button
              type="submit"
              disabled={uploading}
              className="px-6 py-2.5 bg-emerald-800 hover:bg-emerald-900 text-white text-xs font-bold rounded-xl transition shadow-sm disabled:opacity-50 flex items-center gap-2"
            >
              {uploading ? (
                <>
                  <Spinner size="sm" />
                  <span>Submitting Media...</span>
                </>
              ) : (
                <>
                  <Upload className="w-4 h-4" />
                  <span>Submit for Review</span>
                </>
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

export default function GalleryUploadPage() {
  return (
    <Suspense
      fallback={
        <div className="min-h-screen bg-gray-50 flex items-center justify-center p-6">
          <Spinner text="Loading submission portal..." />
        </div>
      }
    >
      <GalleryUploadForm />
    </Suspense>
  );
}

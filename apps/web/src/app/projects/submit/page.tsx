"use client";

import React, { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import {
  FolderKanban,
  Upload,
  X,
  MapPin,
  CheckCircle2,
  AlertCircle,
  ArrowLeft,
  ArrowRight,
  ShieldAlert,
  Image as ImageIcon,
} from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import { State, LGA, CommunityRequest } from "@/lib/types";

export default function SubmitCommunityRequestPage() {
  const router = useRouter();
  const { user, isLoading: authLoading } = useAuth();

  const [states, setStates] = useState<State[]>([]);
  const [lgas, setLgas] = useState<LGA[]>([]);
  const [loadingLgas, setLoadingLgas] = useState(false);

  // Form State
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [category, setCategory] = useState("environmental_sanitation");
  const [stateId, setStateId] = useState("");
  const [lgaId, setLgaId] = useState("");
  const [communityWard, setCommunityWard] = useState("");
  const [locationDetails, setLocationDetails] = useState("");
  const [urgency, setUrgency] = useState<"low" | "medium" | "high" | "critical">("medium");

  // Evidence Photos State
  const [selectedFiles, setSelectedFiles] = useState<File[]>([]);
  const [previews, setPreviews] = useState<string[]>([]);
  const [uploadedUrls, setUploadedUrls] = useState<string[]>([]);
  const [uploading, setUploading] = useState(false);
  const [uploadProgress, setUploadProgress] = useState<string | null>(null);

  // Submission State
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [submittedRequest, setSubmittedRequest] = useState<CommunityRequest | null>(null);

  const categories = [
    { key: "environmental_sanitation", label: "Environmental Sanitation & Cleanliness" },
    { key: "water_and_boreholes", label: "Water Supply & Boreholes" },
    { key: "waste_management", label: "Waste Management & Refuse Clearing" },
    { key: "drainage_and_flooding", label: "Drainage & Flood Prevention" },
    { key: "public_infrastructure", label: "Public Infrastructure & Roads" },
    { key: "community_health_hygiene", label: "Community Health & Hygiene" },
    { key: "education_and_awareness", label: "Civic Education & Awareness" },
    { key: "other", label: "Other Community Needs" },
  ];

  // Auth gate
  useEffect(() => {
    if (!authLoading && !user) {
      router.push("/login?redirect=/projects/submit");
    }
  }, [user, authLoading, router]);

  // Load States
  useEffect(() => {
    async function fetchStates() {
      try {
        const res = await apiRequest<State[]>("/geography/states");
        if (res) setStates(res);
      } catch (err) {
        console.error("Failed to fetch states", err);
      }
    }
    fetchStates();
  }, []);

  // Load LGAs when State changes
  useEffect(() => {
    if (!stateId) {
      setLgas([]);
      setLgaId("");
      return;
    }
    async function fetchLgas() {
      setLoadingLgas(true);
      try {
        const res = await apiRequest<LGA[]>(`/geography/states/${stateId}/lgas`);
        setLgas(res || []);
        if (res && res.length > 0) {
          setLgaId(res[0].id);
        } else {
          setLgaId("");
        }
      } catch (err) {
        console.error("Failed to fetch LGAs", err);
      } finally {
        setLoadingLgas(false);
      }
    }
    fetchLgas();
  }, [stateId]);

  // Handle local file selection
  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!e.target.files) return;
    const files = Array.from(e.target.files);

    const validFiles: File[] = [];
    const newPreviews: string[] = [];

    for (const f of files) {
      if (f.size > 5 * 1024 * 1024) {
        setError(`File "${f.name}" exceeds maximum allowed size of 5 MB.`);
        continue;
      }
      if (!f.type.startsWith("image/")) {
        setError(`File "${f.name}" is not a supported image format.`);
        continue;
      }
      validFiles.push(f);
      newPreviews.push(URL.createObjectURL(f));
    }

    setSelectedFiles((prev) => [...prev, ...validFiles]);
    setPreviews((prev) => [...prev, ...newPreviews]);
  };

  const removeFile = (index: number) => {
    setSelectedFiles((prev) => prev.filter((_, i) => i !== index));
    setPreviews((prev) => prev.filter((_, i) => i !== index));
  };

  // Upload files to Cloudflare R2 via Presigned or Direct
  const uploadEvidenceFiles = async (): Promise<string[]> => {
    if (selectedFiles.length === 0) return [];
    setUploading(true);
    setUploadProgress("Uploading evidence images to Cloudflare R2...");
    const urls: string[] = [];

    for (let i = 0; i < selectedFiles.length; i++) {
      const file = selectedFiles[i];
      setUploadProgress(`Uploading photo ${i + 1} of ${selectedFiles.length}...`);

      try {
        // Try presigned upload first
        const presignRes = await apiRequest<{ upload_url: string; public_url: string }>(
          "/community/requests/presign",
          {
            method: "POST",
            body: JSON.stringify({
              filename: file.name,
              content_type: file.type,
              file_size: file.size,
            }),
          }
        );

        if (presignRes?.upload_url && !presignRes.upload_url.includes("mock-presigned")) {
          const putRes = await fetch(presignRes.upload_url, {
            method: "PUT",
            headers: { "Content-Type": file.type },
            body: file,
          });
          if (putRes.ok) {
            urls.push(presignRes.public_url);
            continue;
          }
        }

        // Direct upload fallback
        const formData = new FormData();
        formData.append("file", file);
        const directRes = await apiRequest<{ public_url: string }>("/community/requests/upload", {
          method: "POST",
          body: formData,
        });
        if (directRes?.public_url) {
          urls.push(directRes.public_url);
        }
      } catch (err: any) {
        console.warn(`Failed to upload ${file.name}, trying direct`, err);
        // Fallback to direct upload
        try {
          const formData = new FormData();
          formData.append("file", file);
          const directRes = await apiRequest<{ public_url: string }>("/community/requests/upload", {
            method: "POST",
            body: formData,
          });
          if (directRes?.public_url) {
            urls.push(directRes.public_url);
          }
        } catch (innerErr) {
          console.error("Direct upload also failed", innerErr);
        }
      }
    }

    setUploading(false);
    setUploadProgress(null);
    return urls;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!title.trim() || !description.trim()) {
      setError("Please provide a title and detailed description of the community need.");
      return;
    }
    if (!stateId || !lgaId) {
      setError("Please select the State and Local Government Area.");
      return;
    }
    if (!communityWard.trim() || !locationDetails.trim()) {
      setError("Please specify the community ward and street location details.");
      return;
    }

    setSubmitting(true);
    try {
      // 1. Upload photos if selected
      const photoUrls = await uploadEvidenceFiles();

      // 2. Submit request
      const payload = {
        title: title.trim(),
        description: description.trim(),
        category,
        state_id: stateId,
        lga_id: lgaId,
        community_ward: communityWard.trim(),
        location_details: locationDetails.trim(),
        estimated_urgency: urgency,
        evidence_image_urls: photoUrls,
      };

      const res = await apiRequest<CommunityRequest>("/community/requests", {
        method: "POST",
        body: JSON.stringify(payload),
      });

      if (res) {
        setSubmittedRequest(res);
      }
    } catch (err: any) {
      console.error("Failed to submit request", err);
      setError(err?.message || "Failed to submit community request. Please try again.");
    } finally {
      setSubmitting(false);
    }
  };

  if (authLoading) {
    return (
      <div className="py-20 text-center text-sm text-gray-500">
        Checking authentication status...
      </div>
    );
  }

  // Success Confirmation Screen
  if (submittedRequest) {
    return (
      <div className="max-w-2xl mx-auto px-4 py-16">
        <div className="p-8 bg-white rounded-2xl border border-gray-200 shadow-sm text-center space-y-6">
          <div className="w-16 h-16 bg-emerald-100 text-emerald-700 rounded-full flex items-center justify-center mx-auto">
            <CheckCircle2 className="w-10 h-10" />
          </div>

          <div className="space-y-2">
            <h1 className="text-2xl font-extrabold text-gray-900">
              Community Request Submitted Successfully!
            </h1>
            <p className="text-sm text-gray-600 max-w-md mx-auto">
              Your report has been queued for verification by the Ratels state coordinator for your area.
            </p>
          </div>

          <div className="p-4 bg-emerald-50 rounded-xl border border-emerald-200 inline-block text-left">
            <div className="text-xs text-emerald-800 font-medium">Tracking Reference:</div>
            <div className="text-lg font-mono font-bold text-emerald-950">
              {submittedRequest.reference}
            </div>
            <div className="text-xs text-emerald-700 pt-1">
              Status: <span className="font-semibold uppercase">Submitted (Pending Review)</span>
            </div>
          </div>

          <div className="flex flex-col sm:flex-row justify-center gap-3 pt-2">
            <Link
              href="/projects/requests"
              className="inline-flex items-center justify-center gap-2 px-6 py-3 text-sm font-semibold rounded-xl text-white bg-emerald-900 hover:bg-emerald-800 shadow-sm transition"
            >
              Track My Submissions
              <ArrowRight className="w-4 h-4" />
            </Link>
            <Link
              href="/projects"
              className="inline-flex items-center justify-center px-6 py-3 text-sm font-medium rounded-xl text-gray-700 bg-gray-100 hover:bg-gray-200 transition"
            >
              Browse Public Projects
            </Link>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-3xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-8">
      <div>
        <Link
          href="/projects"
          className="inline-flex items-center gap-1.5 text-xs font-medium text-emerald-800 hover:text-emerald-900 transition mb-3"
        >
          <ArrowLeft className="w-4 h-4" />
          Back to Community Projects
        </Link>
        <div className="space-y-1">
          <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900 tracking-tight">
            Report a Community Need
          </h1>
          <p className="text-sm text-gray-600 leading-relaxed">
            Submit grassroots sanitation, water, drainage, or infrastructure needs for verification and intervention. Your identity is held privately by Ratels officials.
          </p>
        </div>
      </div>

      {error && (
        <div className="p-4 bg-red-50 border border-red-200 rounded-xl flex items-start gap-3">
          <AlertCircle className="w-5 h-5 text-red-600 shrink-0 mt-0.5" />
          <p className="text-sm text-red-800">{error}</p>
        </div>
      )}

      <form onSubmit={handleSubmit} className="p-6 sm:p-8 bg-white rounded-2xl border border-gray-200 shadow-sm space-y-6">
        {/* Title */}
        <div className="space-y-1.5">
          <label className="text-xs font-bold text-gray-700 uppercase tracking-wider block">
            Request Title *
          </label>
          <input
            type="text"
            required
            placeholder="e.g., Blocked Canal and Drainage Clearance along Market Road"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            className="w-full px-4 py-2.5 text-sm bg-gray-50 border border-gray-300 rounded-xl focus:outline-none focus:ring-2 focus:ring-emerald-500"
          />
        </div>

        {/* Category & Urgency */}
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div className="space-y-1.5">
            <label className="text-xs font-bold text-gray-700 uppercase tracking-wider block">
              Intervention Category *
            </label>
            <select
              value={category}
              onChange={(e) => setCategory(e.target.value)}
              className="w-full px-3 py-2.5 text-sm bg-gray-50 border border-gray-300 rounded-xl focus:outline-none focus:ring-2 focus:ring-emerald-500"
            >
              {categories.map((c) => (
                <option key={c.key} value={c.key}>
                  {c.label}
                </option>
              ))}
            </select>
          </div>

          <div className="space-y-1.5">
            <label className="text-xs font-bold text-gray-700 uppercase tracking-wider block">
              Estimated Urgency *
            </label>
            <select
              value={urgency}
              onChange={(e) => setUrgency(e.target.value as any)}
              className="w-full px-3 py-2.5 text-sm bg-gray-50 border border-gray-300 rounded-xl focus:outline-none focus:ring-2 focus:ring-emerald-500"
            >
              <option value="low">Low — Routine maintenance or planning</option>
              <option value="medium">Medium — Ongoing nuisance / impending hazard</option>
              <option value="high">High — Active flooding, health risk, or disruption</option>
              <option value="critical">Critical — Immediate danger to lives or property</option>
            </select>
          </div>
        </div>

        {/* State & LGA */}
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div className="space-y-1.5">
            <label className="text-xs font-bold text-gray-700 uppercase tracking-wider block">
              State / Territory *
            </label>
            <select
              required
              value={stateId}
              onChange={(e) => setStateId(e.target.value)}
              className="w-full px-3 py-2.5 text-sm bg-gray-50 border border-gray-300 rounded-xl focus:outline-none focus:ring-2 focus:ring-emerald-500"
            >
              <option value="">Select State</option>
              {states.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              ))}
            </select>
          </div>

          <div className="space-y-1.5">
            <label className="text-xs font-bold text-gray-700 uppercase tracking-wider block">
              Local Government Area (LGA) *
            </label>
            <select
              required
              disabled={!stateId || loadingLgas}
              value={lgaId}
              onChange={(e) => setLgaId(e.target.value)}
              className="w-full px-3 py-2.5 text-sm bg-gray-50 border border-gray-300 rounded-xl focus:outline-none focus:ring-2 focus:ring-emerald-500 disabled:opacity-50"
            >
              <option value="">{loadingLgas ? "Loading LGAs..." : "Select LGA"}</option>
              {lgas.map((l) => (
                <option key={l.id} value={l.id}>
                  {l.name}
                </option>
              ))}
            </select>
          </div>
        </div>

        {/* Community Ward & Specific Location */}
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div className="space-y-1.5">
            <label className="text-xs font-bold text-gray-700 uppercase tracking-wider block">
              Community / Ward *
            </label>
            <input
              type="text"
              required
              placeholder="e.g., Ward 05 or Alagomeji Community"
              value={communityWard}
              onChange={(e) => setCommunityWard(e.target.value)}
              className="w-full px-4 py-2.5 text-sm bg-gray-50 border border-gray-300 rounded-xl focus:outline-none focus:ring-2 focus:ring-emerald-500"
            />
          </div>

          <div className="space-y-1.5">
            <label className="text-xs font-bold text-gray-700 uppercase tracking-wider block">
              Street Location & Landmark *
            </label>
            <input
              type="text"
              required
              placeholder="e.g., Opposite Central Mosque, by Old Post Office"
              value={locationDetails}
              onChange={(e) => setLocationDetails(e.target.value)}
              className="w-full px-4 py-2.5 text-sm bg-gray-50 border border-gray-300 rounded-xl focus:outline-none focus:ring-2 focus:ring-emerald-500"
            />
          </div>
        </div>

        {/* Description & Justification */}
        <div className="space-y-1.5">
          <label className="text-xs font-bold text-gray-700 uppercase tracking-wider block">
            Detailed Description & Justification *
          </label>
          <textarea
            required
            rows={5}
            placeholder="Explain the background of the problem, how long it has persisted, the number of residents affected, and any initial volunteer efforts organized..."
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            className="w-full px-4 py-3 text-sm bg-gray-50 border border-gray-300 rounded-xl focus:outline-none focus:ring-2 focus:ring-emerald-500"
          />
        </div>

        {/* Evidence Photos Upload */}
        <div className="space-y-3 pt-2">
          <div className="flex items-center justify-between">
            <label className="text-xs font-bold text-gray-700 uppercase tracking-wider block">
              Supporting Evidence Photographs (Up to 5 images, Max 5 MB each)
            </label>
            <span className="text-xs text-gray-400">JPG, PNG, WebP</span>
          </div>

          {/* Previews */}
          {previews.length > 0 && (
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
              {previews.map((src, i) => (
                <div key={i} className="relative aspect-video rounded-xl overflow-hidden border border-gray-200 group">
                  <img src={src} alt="Evidence preview" className="w-full h-full object-cover" />
                  <button
                    type="button"
                    onClick={() => removeFile(i)}
                    className="absolute top-1.5 right-1.5 p-1 bg-black/70 hover:bg-black text-white rounded-full transition"
                  >
                    <X className="w-3.5 h-3.5" />
                  </button>
                </div>
              ))}
            </div>
          )}

          {previews.length < 5 && (
            <label className="border-2 border-dashed border-gray-300 hover:border-emerald-500 rounded-2xl p-6 flex flex-col items-center justify-center cursor-pointer transition bg-gray-50/50 hover:bg-emerald-50/20">
              <Upload className="w-8 h-8 text-gray-400 mb-2" />
              <span className="text-sm font-semibold text-gray-700">Click to select photos from device</span>
              <span className="text-xs text-gray-400 mt-1">Direct upload to Cloudflare R2 storage</span>
              <input
                type="file"
                multiple
                accept="image/jpeg,image/png,image/webp"
                onChange={handleFileChange}
                className="hidden"
              />
            </label>
          )}

          {uploadProgress && (
            <div className="p-3 bg-emerald-50 border border-emerald-200 rounded-xl text-xs text-emerald-800 flex items-center gap-2">
              <div className="w-4 h-4 border-2 border-emerald-600 border-t-transparent rounded-full animate-spin shrink-0" />
              <span>{uploadProgress}</span>
            </div>
          )}
        </div>

        {/* Privacy Note */}
        <div className="p-4 bg-emerald-50/60 rounded-xl border border-emerald-100 flex items-start gap-2.5 text-xs text-emerald-900">
          <ShieldAlert className="w-4 h-4 text-emerald-700 shrink-0 mt-0.5" />
          <span>
            <strong>Privacy Protection:</strong> Your personal contact information is private and visible only to authorized Ratels review officers for on-ground verification. It will not be displayed on public project listings.
          </span>
        </div>

        {/* Submit Button */}
        <div className="pt-2">
          <button
            type="submit"
            disabled={submitting || uploading}
            className="w-full inline-flex items-center justify-center gap-2 py-3 px-6 text-sm font-bold rounded-xl text-emerald-950 bg-emerald-400 hover:bg-emerald-300 disabled:opacity-50 shadow-md transition"
          >
            {submitting ? (
              <>
                <div className="w-4 h-4 border-2 border-emerald-950 border-t-transparent rounded-full animate-spin" />
                Submitting Request...
              </>
            ) : (
              <>
                Submit Community Request
                <ArrowRight className="w-4 h-4" />
              </>
            )}
          </button>
        </div>
      </form>
    </div>
  );
}

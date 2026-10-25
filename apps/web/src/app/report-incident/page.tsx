"use client";

import React, { useState, useEffect } from "react";
import Link from "next/link";
import {
  ShieldAlert,
  AlertTriangle,
  Lock,
  Eye,
  EyeOff,
  Copy,
  Check,
  Upload,
  FileText,
  X,
  CheckCircle2,
  ArrowRight,
  Info,
  PhoneCall,
  UserCheck,
  UserX,
  Shield,
  LifeBuoy,
} from "lucide-react";
import { apiRequest } from "@/lib/api";
import { State, LGA, SafeguardingPublicConfig, SafeguardingSubmissionResponse } from "@/lib/types";
import { Alert } from "@/components/ui/Alert";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";

const INCIDENT_CATEGORIES = [
  { value: "minor_safeguarding", label: "Child / Minor Safeguarding (Under 18)", isMinor: true },
  { value: "sexual_harassment", label: "Sexual Harassment, Abuse or Exploitation", isMinor: false },
  { value: "bullying_and_intimidation", label: "Bullying, Harassment or Intimidation", isMinor: false },
  { value: "physical_assault", label: "Physical Violence or Assault", isMinor: false },
  { value: "safety_hazard", label: "Severe Safety Hazard / Unsafe Cleanup Conditions", isMinor: false },
  { value: "financial_misconduct", label: "Financial Misconduct, Fraud or Extortion", isMinor: false },
  { value: "discrimination", label: "Discrimination or Exclusion", isMinor: false },
  { value: "leadership_misconduct", label: "Coordinator or Leadership Misconduct", isMinor: false },
  { value: "retaliation", label: "Retaliation for Previous Reporting", isMinor: false },
  { value: "general_grievance", label: "Other Safeguarding Concern / Grievance", isMinor: false },
];

export default function ReportIncidentPage() {
  const [config, setConfig] = useState<SafeguardingPublicConfig | null>(null);
  const [loadingConfig, setLoadingConfig] = useState(true);
  const [states, setStates] = useState<State[]>([]);
  const [lgas, setLgas] = useState<LGA[]>([]);

  // Form State
  const [isAnonymous, setIsAnonymous] = useState(true);
  const [category, setCategory] = useState("minor_safeguarding");
  const [isMinorInvolved, setIsMinorInvolved] = useState(true);
  const [immediateDanger, setImmediateDanger] = useState(false);
  const [incidentDate, setIncidentDate] = useState("");
  const [stateId, setStateId] = useState("");
  const [lgaId, setLgaId] = useState("");
  const [locationDescription, setLocationDescription] = useState("");
  const [narrative, setNarrative] = useState("");

  // Identified Reporter Information
  const [reporterName, setReporterName] = useState("");
  const [reporterEmail, setReporterEmail] = useState("");
  const [reporterPhone, setReporterPhone] = useState("");
  const [reporterRelationship, setReporterRelationship] = useState("member");

  // Evidence Files
  const [evidenceFiles, setEvidenceFiles] = useState<File[]>([]);

  // Submission Status
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [submissionResult, setSubmissionResult] = useState<SafeguardingSubmissionResponse | null>(null);
  const [copiedSecret, setCopiedSecret] = useState(false);
  const [copiedRef, setCopiedRef] = useState(false);

  // Load public safeguarding config and states
  useEffect(() => {
    async function init() {
      try {
        const [cfgData, statesData] = await Promise.all([
          apiRequest<SafeguardingPublicConfig>("/safeguarding/public-config"),
          apiRequest<State[]>("/geography/states").catch(() => []),
        ]);
        setConfig(cfgData);
        setStates(statesData);
      } catch (err: any) {
        console.error("Failed to load safeguarding configuration", err);
        setError("Unable to load the reporting portal configuration. Please try again shortly.");
      } finally {
        setLoadingConfig(false);
      }
    }
    init();
  }, []);

  // Update LGAs when state changes
  useEffect(() => {
    if (!stateId) {
      setLgas([]);
      setLgaId("");
      return;
    }
    async function loadLgas() {
      try {
        const lgaData = await apiRequest<LGA[]>(`/geography/states/${stateId}/lgas`);
        setLgas(lgaData);
      } catch (err) {
        console.error("Failed to load LGAs", err);
        setLgas([]);
      }
    }
    loadLgas();
  }, [stateId]);

  // Sync isMinorInvolved when category changes
  const handleCategoryChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const val = e.target.value;
    setCategory(val);
    const selected = INCIDENT_CATEGORIES.find((c) => c.value === val);
    if (selected && selected.isMinor) {
      setIsMinorInvolved(true);
    }
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!e.target.files) return;
    const filesArray = Array.from(e.target.files);

    // Validate size (max 25MB total per file)
    const oversized = filesArray.some((f) => f.size > 25 * 1024 * 1024);
    if (oversized) {
      alert("Each uploaded evidence file must be smaller than 25MB.");
      return;
    }

    setEvidenceFiles((prev) => [...prev, ...filesArray]);
  };

  const removeFile = (index: number) => {
    setEvidenceFiles((prev) => prev.filter((_, i) => i !== index));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (narrative.trim().length < 20) {
      setError("Please provide a sufficiently detailed description of the incident (at least 20 characters).");
      return;
    }

    if (!isAnonymous && !reporterEmail.trim()) {
      setError("Please provide an email address if you choose to identify yourself, so the safeguarding team may reach you.");
      return;
    }

    setSubmitting(true);
    try {
      const formData = new FormData();
      formData.append("category", category);
      formData.append("description", narrative.trim());
      formData.append("narrative", narrative.trim());
      formData.append("is_anonymous", isAnonymous ? "true" : "false");
      formData.append("is_minor_involved", isMinorInvolved ? "true" : "false");
      formData.append("minor_involved", isMinorInvolved ? "true" : "false");
      formData.append("is_immediate_danger", immediateDanger ? "true" : "false");
      formData.append("immediate_danger", immediateDanger ? "true" : "false");

      if (incidentDate) formData.append("incident_date", incidentDate);
      if (stateId) formData.append("state_id", stateId);
      if (lgaId) formData.append("lga_id", lgaId);
      if (locationDescription) {
        formData.append("location_description", locationDescription.trim());
        formData.append("related_activity", locationDescription.trim());
      }

      if (!isAnonymous) {
        if (reporterName) formData.append("reporter_name", reporterName.trim());
        if (reporterEmail) formData.append("reporter_email", reporterEmail.trim());
        if (reporterPhone) formData.append("reporter_phone", reporterPhone.trim());
        if (reporterRelationship) formData.append("reporter_relationship", reporterRelationship);
      }

      // Attach evidence files
      for (const file of evidenceFiles) {
        formData.append("evidence", file);
      }

      const res = await apiRequest<SafeguardingSubmissionResponse>("/safeguarding/reports", {
        method: "POST",
        body: formData,
      });

      setSubmissionResult(res);
      window.scrollTo({ top: 0, behavior: "smooth" });
    } catch (err: any) {
      setError(err.message || "Failed to submit safeguarding report. Please check your information and try again.");
    } finally {
      setSubmitting(false);
    }
  };

  const copyToClipboard = (text: string, type: "ref" | "sec") => {
    navigator.clipboard.writeText(text);
    if (type === "ref") {
      setCopiedRef(true);
      setTimeout(() => setCopiedRef(false), 3000);
    } else {
      setCopiedSecret(true);
      setTimeout(() => setCopiedSecret(false), 3000);
    }
  };

  if (loadingConfig) {
    return <Spinner text="Loading secure safeguarding portal..." />;
  }

  const isReportingEnabled = config ? (config.reporting_enabled ?? config.is_enabled ?? true) : true;

  // If incident reporting is disabled by server configuration
  if (config && !isReportingEnabled) {
    return (
      <div className="max-w-3xl mx-auto px-4 py-16 text-center space-y-6">
        <div className="w-16 h-16 rounded-full bg-amber-100 text-amber-800 mx-auto flex items-center justify-center">
          <AlertTriangle className="w-8 h-8" />
        </div>
        <h1 className="text-2xl font-black text-gray-900">Incident Reporting Temporarily Offline</h1>
        <p className="text-sm text-gray-600 max-w-lg mx-auto">
          The confidential incident reporting system is currently disabled for maintenance or pending administrative configuration.
        </p>
        <div className="p-5 bg-red-50 border border-red-200 rounded-xl text-left text-xs text-red-900 space-y-2">
          <div className="font-bold flex items-center gap-2 text-sm text-red-950">
            <PhoneCall className="w-4 h-4 text-red-600" />
            Emergency Warning
          </div>
          <p>
            Ratels is not an emergency response service. If you or someone else is in immediate physical danger, medical emergency, or severe harm, please contact local emergency authorities (112) or the Nigeria Police Force immediately.
          </p>
        </div>
        <div>
          <Link
            href="/"
            className="inline-flex items-center px-4 py-2 rounded-lg bg-emerald-800 text-white text-xs font-semibold hover:bg-emerald-700 transition"
          >
            Return to Homepage
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-8">
      {/* Critical Emergency Banner */}
      <div className="bg-red-50 border-2 border-red-300 rounded-2xl p-5 sm:p-6 text-red-950 shadow-sm space-y-3">
        <div className="flex items-start gap-3">
          <div className="p-2 bg-red-600 text-white rounded-lg mt-0.5 shrink-0">
            <AlertTriangle className="w-5 h-5" />
          </div>
          <div className="space-y-1">
            <h2 className="text-sm sm:text-base font-extrabold uppercase tracking-wide text-red-900 flex items-center gap-2">
              Emergency & Safety Notice
            </h2>
            <p className="text-xs sm:text-sm text-red-900 leading-relaxed font-medium">
              <strong>Ratels Worldwide is NOT an emergency response service or law enforcement agency.</strong> If you or anyone else is facing immediate physical violence, a life-threatening medical situation, or imminent danger, please contact the <strong>Nigeria Police Force (112 / 199)</strong> or your local emergency dispatch immediately.
            </p>
          </div>
        </div>
      </div>

      {/* Success Confirmation Modal / Screen */}
      {submissionResult ? (
        <div className="bg-white border-2 border-emerald-600 rounded-3xl p-6 sm:p-10 shadow-lg space-y-8 animate-in fade-in duration-300">
          <div className="text-center space-y-3">
            <div className="w-16 h-16 rounded-full bg-emerald-100 text-emerald-800 mx-auto flex items-center justify-center">
              <CheckCircle2 className="w-10 h-10 text-emerald-700" />
            </div>
            <h1 className="text-2xl sm:text-3xl font-black text-gray-900">
              Confidential Report Received
            </h1>
            <p className="text-sm text-gray-600 max-w-xl mx-auto">
              Your incident report has been securely registered and encrypted in the Ratels safeguarding vault.
            </p>
          </div>

          {/* Secret Credentials Box */}
          <div className="bg-amber-50 border-2 border-amber-300 rounded-2xl p-6 space-y-5">
            <div className="flex items-center gap-2 text-amber-950 font-bold text-sm">
              <Lock className="w-4 h-4 text-amber-700" />
              IMPORTANT: Save Your Case Credentials Immediately
            </div>
            <p className="text-xs text-amber-900 leading-relaxed">
              To guarantee your privacy, <strong>Ratels does not store your secret passkey in plain text</strong>. Because this report may be anonymous, our team has no other way to authenticate you. <strong>If you lose this secret key, you will not be able to check progress or read investigator replies.</strong>
            </p>

            <div className="space-y-4 pt-2">
              {/* Reference Number */}
              <div>
                <label className="block text-xs font-bold uppercase tracking-wider text-gray-600 mb-1">
                  Case Reference Number
                </label>
                <div className="flex items-center gap-2">
                  <code className="block flex-1 p-3 bg-white border border-amber-300 rounded-lg font-mono text-sm sm:text-base font-extrabold text-gray-900">
                    {submissionResult.reference_number || submissionResult.public_reference}
                  </code>
                  <button
                    type="button"
                    onClick={() => copyToClipboard(submissionResult.reference_number || submissionResult.public_reference, "ref")}
                    className="p-3 bg-white border border-amber-300 hover:bg-amber-100 text-amber-900 rounded-lg font-medium text-xs flex items-center gap-1.5 transition shrink-0"
                  >
                    {copiedRef ? <Check className="w-4 h-4 text-emerald-600" /> : <Copy className="w-4 h-4" />}
                    {copiedRef ? "Copied" : "Copy"}
                  </button>
                </div>
              </div>

              {/* Follow-Up Secret Key */}
              <div>
                <label className="block text-xs font-bold uppercase tracking-wider text-gray-600 mb-1">
                  Private Follow-Up Secret Passkey
                </label>
                <div className="flex items-center gap-2">
                  <code className="block flex-1 p-3 bg-white border border-amber-300 rounded-lg font-mono text-xs sm:text-sm font-extrabold text-red-700 break-all select-all">
                    {submissionResult.follow_up_secret}
                  </code>
                  <button
                    type="button"
                    onClick={() => copyToClipboard(submissionResult.follow_up_secret, "sec")}
                    className="p-3 bg-white border border-amber-300 hover:bg-amber-100 text-amber-900 rounded-lg font-medium text-xs flex items-center gap-1.5 transition shrink-0"
                  >
                    {copiedSecret ? <Check className="w-4 h-4 text-emerald-600" /> : <Copy className="w-4 h-4" />}
                    {copiedSecret ? "Copied" : "Copy"}
                  </button>
                </div>
              </div>
            </div>
          </div>

          <div className="p-4 bg-gray-50 border border-gray-200 rounded-xl text-xs text-gray-600 space-y-1">
            <span className="font-bold text-gray-800">Next Steps:</span>
            <p>
              An appointed safeguarding officer will triage this matter according to our duty-of-care protocols. You can use your Reference Number and Secret Passkey to view confidential status updates or communicate bidirectionally with the review team without exposing your personal identity.
            </p>
          </div>

          <div className="flex flex-col sm:flex-row gap-3 justify-center pt-2">
            <Link
              href={`/report-incident/status?ref=${encodeURIComponent(submissionResult.reference_number || submissionResult.public_reference)}&sec=${encodeURIComponent(submissionResult.follow_up_secret)}`}
              className="inline-flex items-center justify-center px-6 py-3 rounded-xl bg-emerald-800 hover:bg-emerald-700 text-white font-bold text-xs uppercase tracking-wider transition shadow-sm"
            >
              Go to Status & Messaging Portal
              <ArrowRight className="w-4 h-4 ml-2" />
            </Link>
            <button
              type="button"
              onClick={() => {
                setSubmissionResult(null);
                setNarrative("");
                setEvidenceFiles([]);
              }}
              className="px-6 py-3 rounded-xl border border-gray-300 hover:bg-gray-50 text-gray-700 font-semibold text-xs transition"
            >
              Submit Another Report
            </button>
          </div>
        </div>
      ) : (
        /* Report Submission Form */
        <div className="bg-white rounded-3xl border border-gray-200 shadow-sm overflow-hidden">
          {/* Header */}
          <div className="bg-emerald-950 text-white p-6 sm:p-8 space-y-2">
            <div className="flex items-center gap-2">
              <span className="p-1.5 bg-emerald-800 rounded-md text-emerald-300">
                <Shield className="w-4 h-4" />
              </span>
              <span className="text-xs uppercase font-extrabold tracking-wider text-emerald-400">
                Confidential Incident Portal
              </span>
            </div>
            <h1 className="text-2xl sm:text-3xl font-black">
              Report a Safeguarding or Conduct Concern
            </h1>
            <p className="text-xs sm:text-sm text-emerald-200 max-w-2xl leading-relaxed">
              The Ratels Worldwide Movement enforces zero tolerance for harassment, exploitation, child endangerment, financial corruption, and retaliation. You may choose to report completely anonymously.
            </p>
          </div>

          <form onSubmit={handleSubmit} className="p-6 sm:p-8 space-y-8">
            {error && <Alert type="error">{error}</Alert>}

            {/* Reporting Mode: Anonymous vs Identified */}
            <div className="space-y-4">
              <label className="block text-sm font-bold text-gray-900">
                1. Reporting Identity Option
              </label>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <button
                  type="button"
                  onClick={() => setIsAnonymous(true)}
                  className={`p-4 rounded-xl border-2 text-left transition flex items-start gap-3 ${
                    isAnonymous
                      ? "border-emerald-600 bg-emerald-50/60 text-emerald-950 ring-2 ring-emerald-600/20"
                      : "border-gray-200 bg-white text-gray-700 hover:border-gray-300"
                  }`}
                >
                  <div
                    className={`p-2 rounded-lg ${
                      isAnonymous ? "bg-emerald-700 text-white" : "bg-gray-100 text-gray-600"
                    }`}
                  >
                    <UserX className="w-5 h-5" />
                  </div>
                  <div>
                    <div className="font-bold text-sm">Submit Anonymously</div>
                    <div className="text-xs text-gray-500 mt-1 leading-relaxed">
                      Your identity and contact details will not be collected or linked. You will receive a private passkey to view updates.
                    </div>
                  </div>
                </button>

                <button
                  type="button"
                  onClick={() => setIsAnonymous(false)}
                  className={`p-4 rounded-xl border-2 text-left transition flex items-start gap-3 ${
                    !isAnonymous
                      ? "border-emerald-600 bg-emerald-50/60 text-emerald-950 ring-2 ring-emerald-600/20"
                      : "border-gray-200 bg-white text-gray-700 hover:border-gray-300"
                  }`}
                >
                  <div
                    className={`p-2 rounded-lg ${
                      !isAnonymous ? "bg-emerald-700 text-white" : "bg-gray-100 text-gray-600"
                    }`}
                  >
                    <UserCheck className="w-5 h-5" />
                  </div>
                  <div>
                    <div className="font-bold text-sm">Identify Myself</div>
                    <div className="text-xs text-gray-500 mt-1 leading-relaxed">
                      Provide contact details so safeguarding officers can reach you directly for confidential follow-up questions.
                    </div>
                  </div>
                </button>
              </div>

              {/* Identified Fields */}
              {!isAnonymous && (
                <div className="bg-gray-50 border border-gray-200 rounded-2xl p-5 space-y-4 animate-in fade-in duration-200">
                  <div className="text-xs font-bold text-gray-800 uppercase tracking-wider">
                    Your Contact Information (Protected & Encrypted)
                  </div>
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                    <div>
                      <label className="block text-xs font-semibold text-gray-700 mb-1">
                        Full Name / Preferred Name
                      </label>
                      <input
                        type="text"
                        value={reporterName}
                        onChange={(e) => setReporterName(e.target.value)}
                        placeholder="e.g. Comrade Chidi"
                        className="w-full px-3 py-2 text-xs border border-gray-300 rounded-lg focus:ring-2 focus:ring-emerald-600 focus:outline-none"
                      />
                    </div>

                    <div>
                      <label className="block text-xs font-semibold text-gray-700 mb-1">
                        Email Address <span className="text-red-500">*</span>
                      </label>
                      <input
                        type="email"
                        required={!isAnonymous}
                        value={reporterEmail}
                        onChange={(e) => setReporterEmail(e.target.value)}
                        placeholder="e.g. reporter@example.com"
                        className="w-full px-3 py-2 text-xs border border-gray-300 rounded-lg focus:ring-2 focus:ring-emerald-600 focus:outline-none"
                      />
                    </div>

                    <div>
                      <label className="block text-xs font-semibold text-gray-700 mb-1">
                        Phone Number (Optional)
                      </label>
                      <input
                        type="tel"
                        value={reporterPhone}
                        onChange={(e) => setReporterPhone(e.target.value)}
                        placeholder="e.g. +234 800 000 0000"
                        className="w-full px-3 py-2 text-xs border border-gray-300 rounded-lg focus:ring-2 focus:ring-emerald-600 focus:outline-none"
                      />
                    </div>

                    <div>
                      <label className="block text-xs font-semibold text-gray-700 mb-1">
                        Relationship to Movement
                      </label>
                      <select
                        value={reporterRelationship}
                        onChange={(e) => setReporterRelationship(e.target.value)}
                        className="w-full px-3 py-2 text-xs border border-gray-300 rounded-lg focus:ring-2 focus:ring-emerald-600 focus:outline-none bg-white"
                      >
                        <option value="member">Registered Ratels Member</option>
                        <option value="volunteer">Volunteer Cleanup Participant</option>
                        <option value="coordinator">Coordinator / Appointed Official</option>
                        <option value="beneficiary">Community Beneficiary</option>
                        <option value="vendor">Vendor / Marketplace Merchant</option>
                        <option value="public">Member of the General Public</option>
                      </select>
                    </div>
                  </div>
                </div>
              )}
            </div>

            {/* Category & Special Protection Flags */}
            <div className="space-y-4 pt-2">
              <label className="block text-sm font-bold text-gray-900">
                2. Nature of Concern
              </label>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Primary Category <span className="text-red-500">*</span>
                </label>
                <select
                  value={category}
                  onChange={handleCategoryChange}
                  className="w-full px-3 py-2.5 text-xs sm:text-sm border border-gray-300 rounded-lg focus:ring-2 focus:ring-emerald-600 focus:outline-none bg-white"
                >
                  {INCIDENT_CATEGORIES.map((cat) => (
                    <option key={cat.value} value={cat.value}>
                      {cat.label}
                    </option>
                  ))}
                </select>
              </div>

              {/* Special Protection Checkboxes */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 pt-2">
                <label className="flex items-start gap-3 p-4 bg-amber-50/70 border border-amber-200 rounded-xl cursor-pointer hover:bg-amber-50 transition">
                  <input
                    type="checkbox"
                    checked={isMinorInvolved}
                    onChange={(e) => setIsMinorInvolved(e.target.checked)}
                    className="mt-1 h-4 w-4 text-emerald-700 border-gray-300 rounded focus:ring-emerald-500"
                  />
                  <div>
                    <span className="font-bold text-xs text-amber-950 block">
                      Does this incident involve a minor? (Under 18)
                    </span>
                    <span className="text-[11px] text-amber-800 leading-normal block mt-0.5">
                      Triggers specialized Child Safeguarding protocols and extended 7-year audit retention.
                    </span>
                  </div>
                </label>

                <label className="flex items-start gap-3 p-4 bg-red-50/70 border border-red-200 rounded-xl cursor-pointer hover:bg-red-50 transition">
                  <input
                    type="checkbox"
                    checked={immediateDanger}
                    onChange={(e) => setImmediateDanger(e.target.checked)}
                    className="mt-1 h-4 w-4 text-red-700 border-gray-300 rounded focus:ring-red-500"
                  />
                  <div>
                    <span className="font-bold text-xs text-red-950 block">
                      Immediate or Ongoing Physical Safety Hazard?
                    </span>
                    <span className="text-[11px] text-red-800 leading-normal block mt-0.5">
                      Immediately prioritizes report triage to Urgent for rapid officer attention.
                    </span>
                  </div>
                </label>
              </div>
            </div>

            {/* Time & Location Information */}
            <div className="space-y-4 pt-2">
              <label className="block text-sm font-bold text-gray-900">
                3. Date & Geographic Location (Optional)
              </label>

              <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                <div>
                  <label className="block text-xs font-semibold text-gray-700 mb-1">
                    Date of Incident (Approximate)
                  </label>
                  <input
                    type="date"
                    value={incidentDate}
                    onChange={(e) => setIncidentDate(e.target.value)}
                    className="w-full px-3 py-2 text-xs border border-gray-300 rounded-lg focus:ring-2 focus:ring-emerald-600 focus:outline-none"
                  />
                </div>

                <div>
                  <label className="block text-xs font-semibold text-gray-700 mb-1">
                    State
                  </label>
                  <select
                    value={stateId}
                    onChange={(e) => setStateId(e.target.value)}
                    className="w-full px-3 py-2 text-xs border border-gray-300 rounded-lg focus:ring-2 focus:ring-emerald-600 focus:outline-none bg-white"
                  >
                    <option value="">Select State (Optional)</option>
                    {states.map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.name}
                      </option>
                    ))}
                  </select>
                </div>

                <div>
                  <label className="block text-xs font-semibold text-gray-700 mb-1">
                    Local Government Area (LGA)
                  </label>
                  <select
                    value={lgaId}
                    disabled={!stateId || lgas.length === 0}
                    onChange={(e) => setLgaId(e.target.value)}
                    className="w-full px-3 py-2 text-xs border border-gray-300 rounded-lg focus:ring-2 focus:ring-emerald-600 focus:outline-none bg-white disabled:bg-gray-100"
                  >
                    <option value="">Select LGA (Optional)</option>
                    {lgas.map((l) => (
                      <option key={l.id} value={l.id}>
                        {l.name}
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Specific Venue or Location Description
                </label>
                <input
                  type="text"
                  value={locationDescription}
                  onChange={(e) => setLocationDescription(e.target.value)}
                  placeholder="e.g. Ikeja Bus Terminal Cleanup Event, or Ratels WhatsApp Group"
                  className="w-full px-3 py-2 text-xs border border-gray-300 rounded-lg focus:ring-2 focus:ring-emerald-600 focus:outline-none"
                />
              </div>
            </div>

            {/* Narrative Description */}
            <div className="space-y-2 pt-2">
              <label className="block text-sm font-bold text-gray-900">
                4. Incident Narrative <span className="text-red-500">*</span>
              </label>
              <p className="text-xs text-gray-500">
                Please describe what happened in as much detail as you are comfortable sharing (who was involved, what occurred, witness details, impact). This field is encrypted with AES-256-GCM.
              </p>
              <textarea
                required
                rows={6}
                value={narrative}
                onChange={(e) => setNarrative(e.target.value)}
                placeholder="Describe the incident, names or descriptions of parties involved, dates, sequence of events..."
                className="w-full p-3.5 text-xs sm:text-sm border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-600 focus:outline-none font-sans"
              />
              <div className="text-[11px] text-gray-500 text-right">
                {narrative.length} characters (minimum 20 characters required)
              </div>
            </div>

            {/* Evidence File Upload */}
            <div className="space-y-3 pt-2">
              <label className="block text-sm font-bold text-gray-900">
                5. Supporting Evidence (Optional)
              </label>
              <p className="text-xs text-gray-500">
                You may upload photos, audio recordings, documents, or screenshots. Files are stored in a private, encrypted vault and never exposed publicly. Max 25MB per file (JPG, PNG, PDF, MP4, WebM).
              </p>

              <div className="border-2 border-dashed border-gray-300 hover:border-emerald-500 rounded-2xl p-6 text-center transition bg-gray-50/50">
                <input
                  type="file"
                  id="evidence-upload"
                  multiple
                  onChange={handleFileChange}
                  accept="image/jpeg,image/png,image/webp,application/pdf,video/mp4,video/webm"
                  className="hidden"
                />
                <label
                  htmlFor="evidence-upload"
                  className="cursor-pointer flex flex-col items-center justify-center space-y-2"
                >
                  <div className="p-3 bg-white border border-gray-200 rounded-xl shadow-sm text-gray-600">
                    <Upload className="w-5 h-5 text-emerald-700" />
                  </div>
                  <span className="text-xs font-bold text-emerald-900">
                    Click to select files or drag and drop
                  </span>
                  <span className="text-[11px] text-gray-400">
                    Supported: JPG, PNG, PDF, MP4 (up to 25MB each)
                  </span>
                </label>
              </div>

              {/* Uploaded File List */}
              {evidenceFiles.length > 0 && (
                <div className="space-y-2 pt-2">
                  <div className="text-xs font-bold text-gray-700">
                    Attached Files ({evidenceFiles.length})
                  </div>
                  <div className="space-y-1.5">
                    {evidenceFiles.map((f, idx) => (
                      <div
                        key={idx}
                        className="flex items-center justify-between p-2.5 bg-gray-50 border border-gray-200 rounded-lg text-xs"
                      >
                        <div className="flex items-center gap-2 truncate">
                          <FileText className="w-4 h-4 text-emerald-700 shrink-0" />
                          <span className="font-medium text-gray-800 truncate">{f.name}</span>
                          <span className="text-gray-400 text-[10px]">
                            ({(f.size / 1024 / 1024).toFixed(2)} MB)
                          </span>
                        </div>
                        <button
                          type="button"
                          onClick={() => removeFile(idx)}
                          className="p-1 text-gray-400 hover:text-red-600 rounded transition"
                        >
                          <X className="w-4 h-4" />
                        </button>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>

            {/* Submission Actions */}
            <div className="pt-4 border-t border-gray-200 flex flex-col sm:flex-row items-center justify-between gap-4">
              <div className="flex items-center gap-2 text-xs text-gray-500">
                <Lock className="w-4 h-4 text-emerald-700 shrink-0" />
                <span>Protected by AES-256 authenticated encryption & audit logging</span>
              </div>

              <div className="flex items-center gap-3 w-full sm:w-auto">
                <Link
                  href="/report-incident/status"
                  className="px-4 py-2.5 rounded-xl border border-gray-300 hover:bg-gray-50 text-gray-700 text-xs font-semibold transition text-center"
                >
                  Lookup Existing Report
                </Link>
                <button
                  type="submit"
                  disabled={submitting}
                  className="flex-1 sm:flex-initial px-6 py-2.5 rounded-xl bg-emerald-800 hover:bg-emerald-700 disabled:bg-emerald-400 text-white text-xs font-bold uppercase tracking-wider transition shadow-sm"
                >
                  {submitting ? "Encrypting & Submitting..." : "Submit Confidential Report"}
                </button>
              </div>
            </div>
          </form>
        </div>
      )}
    </div>
  );
}

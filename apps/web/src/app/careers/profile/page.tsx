"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  UserCheck,
  Briefcase,
  GraduationCap,
  Award,
  ShieldCheck,
  Save,
  Plus,
  Trash2,
  FileText,
  AlertCircle,
  CheckCircle2,
  ArrowLeft,
  Lock,
  Layers,
} from "lucide-react";
import { apiRequest } from "@/lib/api";
import { CareersCandidateProfile, safeJsonParse } from "@/lib/types";
import { useAuth } from "@/lib/auth-context";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";
import { Badge } from "@/components/ui/Badge";

const SECTORS = [
  { id: "sanitation_environment", label: "Sanitation & Environment" },
  { id: "logistics_supply_chain", label: "Logistics & Supply Chain" },
  { id: "safety_security", label: "Health, Safety & PPE" },
  { id: "operations_management", label: "Operations & Management" },
  { id: "community_advocacy", label: "Community & Advocacy" },
  { id: "administration_hr", label: "Administration & HR" },
  { id: "technology_data", label: "Technology & Data" },
];

export default function CandidateProfilePage() {
  const router = useRouter();
  const { user, isLoading: authLoading } = useAuth();

  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [statusMessage, setStatusMessage] = useState<{ type: "success" | "error"; text: string } | null>(
    null
  );

  // Form Fields
  const [headline, setHeadline] = useState("");
  const [summary, setSummary] = useState("");
  const [skillsText, setSkillsText] = useState("");
  const [preferredWorkplace, setPreferredWorkplace] = useState<"any" | "on_site" | "remote" | "hybrid">("any");
  const [preferredCategories, setPreferredCategories] = useState<string[]>([]);
  const [resumeUrl, setResumeUrl] = useState("");
  const [resumeFilename, setResumeFilename] = useState("");
  const [isSearchable, setIsSearchable] = useState(true);

  // Dynamic Array Fields
  const [experience, setExperience] = useState<
    Array<{
      company: string;
      title: string;
      location?: string;
      start_date?: string;
      end_date?: string;
      current?: boolean;
      description?: string;
    }>
  >([]);

  const [education, setEducation] = useState<
    Array<{
      institution: string;
      degree: string;
      field_of_study?: string;
      start_year?: number;
      end_year?: number;
    }>
  >([]);

  const [certifications, setCertifications] = useState<
    Array<{
      name: string;
      issuing_org: string;
      year?: number;
      credential_id?: string;
    }>
  >([]);

  useEffect(() => {
    if (!authLoading && !user) {
      router.push("/login?redirect=/careers/profile");
      return;
    }

    if (user) {
      loadProfile();
    }
  }, [user, authLoading]);

  const loadProfile = async () => {
    setLoading(true);
    setStatusMessage(null);
    try {
      const data = await apiRequest<CareersCandidateProfile>("/careers/candidate/profile");
      if (data) {
        setHeadline(data.professional_headline || "");
        setSummary(data.summary || "");

        const skillsArr = safeJsonParse<string[]>(data.skills, []);
        setSkillsText(skillsArr.join(", "));

        setPreferredWorkplace(data.preferred_workplace_type || "any");

        const catsArr = safeJsonParse<string[]>(data.preferred_categories, []);
        setPreferredCategories(catsArr);

        setResumeUrl(data.resume_url || "");
        setResumeFilename(data.resume_filename || "");
        setIsSearchable(data.is_searchable_by_verified_employers ?? true);

        const expArr = safeJsonParse<any[]>(data.experience, []);
        setExperience(expArr);

        const eduArr = safeJsonParse<any[]>(data.education, []);
        setEducation(eduArr);

        const certArr = safeJsonParse<any[]>(data.certifications, []);
        setCertifications(certArr);
      }
    } catch (err: any) {
      setStatusMessage({
        type: "error",
        text: err.message || "Failed to load profile. Please refresh.",
      });
    } finally {
      setLoading(false);
    }
  };

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setStatusMessage(null);

    const skills = skillsText
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);

    try {
      await apiRequest("/careers/candidate/profile", {
        method: "PUT",
        body: JSON.stringify({
          professional_headline: headline.trim(),
          summary: summary.trim(),
          skills,
          experience,
          education,
          certifications,
          preferred_categories: preferredCategories,
          preferred_workplace_type: preferredWorkplace,
          resume_url: resumeUrl.trim(),
          resume_filename: resumeFilename.trim(),
          is_searchable_by_verified_employers: isSearchable,
        }),
      });

      setStatusMessage({
        type: "success",
        text: "Your professional profile has been saved successfully.",
      });
      window.scrollTo({ top: 0, behavior: "smooth" });
    } catch (err: any) {
      setStatusMessage({
        type: "error",
        text: err.message || "Failed to save profile. Please review the form.",
      });
    } finally {
      setSaving(false);
    }
  };

  const toggleCategory = (catId: string) => {
    setPreferredCategories((prev) =>
      prev.includes(catId) ? prev.filter((c) => c !== catId) : [...prev, catId]
    );
  };

  // Dynamic Array Modifiers
  const addExperience = () => {
    setExperience((prev) => [
      ...prev,
      { company: "", title: "", location: "", start_date: "", end_date: "", current: false, description: "" },
    ]);
  };

  const updateExperience = (index: number, field: string, value: any) => {
    setExperience((prev) => {
      const updated = [...prev];
      updated[index] = { ...updated[index], [field]: value };
      return updated;
    });
  };

  const removeExperience = (index: number) => {
    setExperience((prev) => prev.filter((_, i) => i !== index));
  };

  const addEducation = () => {
    setEducation((prev) => [
      ...prev,
      { institution: "", degree: "", field_of_study: "", start_year: new Date().getFullYear(), end_year: new Date().getFullYear() },
    ]);
  };

  const updateEducation = (index: number, field: string, value: any) => {
    setEducation((prev) => {
      const updated = [...prev];
      updated[index] = { ...updated[index], [field]: value };
      return updated;
    });
  };

  const removeEducation = (index: number) => {
    setEducation((prev) => prev.filter((_, i) => i !== index));
  };

  const addCertification = () => {
    setCertifications((prev) => [
      ...prev,
      { name: "", issuing_org: "", year: new Date().getFullYear(), credential_id: "" },
    ]);
  };

  const updateCertification = (index: number, field: string, value: any) => {
    setCertifications((prev) => {
      const updated = [...prev];
      updated[index] = { ...updated[index], [field]: value };
      return updated;
    });
  };

  const removeCertification = (index: number) => {
    setCertifications((prev) => prev.filter((_, i) => i !== index));
  };

  if (authLoading || loading) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center py-20">
        <Spinner text="Loading candidate profile..." />
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50 pb-20">
      {/* Top Header */}
      <section className="bg-emerald-900 text-white py-10 px-4 sm:px-6 lg:px-8 border-b border-emerald-800">
        <div className="max-w-5xl mx-auto">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div>
              <Link
                href="/careers"
                className="inline-flex items-center gap-1.5 text-xs text-emerald-200 hover:text-white mb-2 transition"
              >
                <ArrowLeft className="w-3.5 h-3.5" />
                Back to Careers Discovery
              </Link>
              <h1 className="text-2xl sm:text-3xl font-extrabold tracking-tight">
                Candidate Professional Profile
              </h1>
              <p className="text-emerald-100 text-sm mt-1">
                Maintain your verified credentials, skills, and work history.
              </p>
            </div>

            <div className="flex items-center gap-2">
              <Link
                href="/careers/applications"
                className="px-3.5 py-2 rounded-lg bg-emerald-800 hover:bg-emerald-700 text-xs font-medium text-white transition border border-emerald-700"
              >
                My Applications
              </Link>
            </div>
          </div>
        </div>
      </section>

      {/* Main Form Container */}
      <div className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 pt-8">
        {/* Privacy Callout Banner */}
        <div className="bg-white rounded-xl border border-emerald-100 p-4 mb-6 shadow-sm flex items-start gap-3">
          <Lock className="w-5 h-5 text-emerald-700 flex-shrink-0 mt-0.5" />
          <div className="text-xs text-gray-600 leading-relaxed">
            <span className="font-semibold text-gray-900">Privacy Guarantee:</span> Your profile is
            strictly confidential. Recruiters and employers can only view your credentials, CV, and
            contact details when you explicitly submit an application for their posted opportunity.
          </div>
        </div>

        {statusMessage && (
          <div className="mb-6">
            <Alert type={statusMessage.type} title={statusMessage.type === "success" ? "Saved" : "Notice"}>
              {statusMessage.text}
            </Alert>
          </div>
        )}

        <form onSubmit={handleSave} className="space-y-8">
          {/* Section 1: Professional Overview */}
          <div className="bg-white rounded-xl border border-gray-200 p-6 sm:p-8 shadow-sm space-y-6">
            <h2 className="text-lg font-bold text-gray-900 flex items-center gap-2 pb-2 border-b border-gray-100">
              <UserCheck className="w-5 h-5 text-emerald-700" />
              Professional Overview
            </h2>

            <div>
              <label className="block text-xs font-semibold text-gray-700 uppercase tracking-wider mb-1">
                Professional Headline
              </label>
              <input
                type="text"
                placeholder="e.g. Lead Environmental Safety Officer • OSHA Certified"
                value={headline}
                onChange={(e) => setHeadline(e.target.value)}
                className="w-full text-sm p-3 rounded-lg border border-gray-300 focus:ring-2 focus:ring-emerald-700 focus:border-emerald-700"
              />
              <span className="text-[11px] text-gray-500 mt-1 block">
                A concise summary of your primary professional discipline and qualifications.
              </span>
            </div>

            <div>
              <label className="block text-xs font-semibold text-gray-700 uppercase tracking-wider mb-1">
                Executive Summary & Background
              </label>
              <textarea
                rows={4}
                placeholder="Highlight your background, core competencies, accomplishments, and career goals..."
                value={summary}
                onChange={(e) => setSummary(e.target.value)}
                className="w-full text-sm p-3 rounded-lg border border-gray-300 focus:ring-2 focus:ring-emerald-700 focus:border-emerald-700"
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-gray-700 uppercase tracking-wider mb-1">
                Skills & Technical Competencies (comma separated)
              </label>
              <input
                type="text"
                placeholder="e.g. Environmental Auditing, Waste Management, OSHA 30, First Aid, Supply Logistics"
                value={skillsText}
                onChange={(e) => setSkillsText(e.target.value)}
                className="w-full text-sm p-3 rounded-lg border border-gray-300 focus:ring-2 focus:ring-emerald-700 focus:border-emerald-700"
              />
              <span className="text-[11px] text-gray-500 mt-1 block">
                These skills are matched explainably against job requirements when calculating match scores.
              </span>
            </div>
          </div>

          {/* Section 2: Preferences & Documents */}
          <div className="bg-white rounded-xl border border-gray-200 p-6 sm:p-8 shadow-sm space-y-6">
            <h2 className="text-lg font-bold text-gray-900 flex items-center gap-2 pb-2 border-b border-gray-100">
              <FileText className="w-5 h-5 text-emerald-700" />
              Preferences & Document Attachment
            </h2>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-6">
              <div>
                <label className="block text-xs font-semibold text-gray-700 uppercase tracking-wider mb-1">
                  Preferred Workplace Type
                </label>
                <select
                  value={preferredWorkplace}
                  onChange={(e: any) => setPreferredWorkplace(e.target.value)}
                  className="w-full text-sm p-2.5 rounded-lg border border-gray-300 focus:ring-emerald-700 focus:border-emerald-700"
                >
                  <option value="any">Any (On-site, Remote, or Hybrid)</option>
                  <option value="on_site">On-site only</option>
                  <option value="remote">Remote only</option>
                  <option value="hybrid">Hybrid</option>
                </select>
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 uppercase tracking-wider mb-1">
                  Curriculum Vitae / Résumé Document Link
                </label>
                <input
                  type="text"
                  placeholder="Link to CV (e.g. Google Drive, Cloudinary PDF)"
                  value={resumeUrl}
                  onChange={(e) => setResumeUrl(e.target.value)}
                  className="w-full text-sm p-2.5 rounded-lg border border-gray-300 focus:ring-emerald-700 focus:border-emerald-700"
                />
              </div>
            </div>

            <div>
              <label className="block text-xs font-semibold text-gray-700 uppercase tracking-wider mb-2">
                Preferred Sectors & Categories
              </label>
              <div className="flex flex-wrap gap-2">
                {SECTORS.map((sec) => (
                  <button
                    type="button"
                    key={sec.id}
                    onClick={() => toggleCategory(sec.id)}
                    className={`text-xs px-3 py-1.5 rounded-full font-medium transition ${
                      preferredCategories.includes(sec.id)
                        ? "bg-emerald-900 text-white"
                        : "bg-gray-100 text-gray-700 hover:bg-gray-200"
                    }`}
                  >
                    {sec.label}
                  </button>
                ))}
              </div>
            </div>
          </div>

          {/* Section 3: Work Experience */}
          <div className="bg-white rounded-xl border border-gray-200 p-6 sm:p-8 shadow-sm space-y-6">
            <div className="flex items-center justify-between pb-2 border-b border-gray-100">
              <h2 className="text-lg font-bold text-gray-900 flex items-center gap-2">
                <Briefcase className="w-5 h-5 text-emerald-700" />
                Work Experience
              </h2>
              <button
                type="button"
                onClick={addExperience}
                className="text-xs font-medium text-emerald-800 hover:text-emerald-900 flex items-center gap-1 bg-emerald-50 px-2.5 py-1.5 rounded-lg border border-emerald-200 transition"
              >
                <Plus className="w-3.5 h-3.5" />
                Add Experience
              </button>
            </div>

            {experience.length === 0 ? (
              <p className="text-xs text-gray-500 py-3 italic">
                No work experience recorded yet. Click &apos;Add Experience&apos; to list past roles.
              </p>
            ) : (
              <div className="space-y-4">
                {experience.map((exp, idx) => (
                  <div key={idx} className="p-4 rounded-xl border border-gray-200 bg-gray-50 space-y-3 relative">
                    <button
                      type="button"
                      onClick={() => removeExperience(idx)}
                      className="absolute top-3 right-3 text-gray-400 hover:text-rose-600 transition"
                      title="Remove entry"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>

                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pr-8">
                      <div>
                        <label className="block text-[11px] font-semibold text-gray-600 uppercase">
                          Job Title *
                        </label>
                        <input
                          type="text"
                          required
                          placeholder="e.g. Sanitation Field Supervisor"
                          value={exp.title}
                          onChange={(e) => updateExperience(idx, "title", e.target.value)}
                          className="w-full text-xs p-2 rounded-lg border border-gray-300 bg-white"
                        />
                      </div>
                      <div>
                        <label className="block text-[11px] font-semibold text-gray-600 uppercase">
                          Organization / Company *
                        </label>
                        <input
                          type="text"
                          required
                          placeholder="e.g. Lagos Waste Management Authority"
                          value={exp.company}
                          onChange={(e) => updateExperience(idx, "company", e.target.value)}
                          className="w-full text-xs p-2 rounded-lg border border-gray-300 bg-white"
                        />
                      </div>
                    </div>

                    <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                      <div>
                        <label className="block text-[11px] font-semibold text-gray-600 uppercase">
                          Location
                        </label>
                        <input
                          type="text"
                          placeholder="e.g. Ikeja, Lagos"
                          value={exp.location || ""}
                          onChange={(e) => updateExperience(idx, "location", e.target.value)}
                          className="w-full text-xs p-2 rounded-lg border border-gray-300 bg-white"
                        />
                      </div>
                      <div>
                        <label className="block text-[11px] font-semibold text-gray-600 uppercase">
                          Start Date
                        </label>
                        <input
                          type="text"
                          placeholder="e.g. Jan 2022"
                          value={exp.start_date || ""}
                          onChange={(e) => updateExperience(idx, "start_date", e.target.value)}
                          className="w-full text-xs p-2 rounded-lg border border-gray-300 bg-white"
                        />
                      </div>
                      <div>
                        <label className="block text-[11px] font-semibold text-gray-600 uppercase">
                          End Date
                        </label>
                        <input
                          type="text"
                          placeholder="e.g. Present"
                          value={exp.end_date || ""}
                          onChange={(e) => updateExperience(idx, "end_date", e.target.value)}
                          className="w-full text-xs p-2 rounded-lg border border-gray-300 bg-white"
                        />
                      </div>
                    </div>

                    <div>
                      <label className="block text-[11px] font-semibold text-gray-600 uppercase">
                        Description & Achievements
                      </label>
                      <textarea
                        rows={2}
                        placeholder="Brief summary of duties and accomplishments..."
                        value={exp.description || ""}
                        onChange={(e) => updateExperience(idx, "description", e.target.value)}
                        className="w-full text-xs p-2 rounded-lg border border-gray-300 bg-white"
                      />
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Section 4: Education & Certifications */}
          <div className="bg-white rounded-xl border border-gray-200 p-6 sm:p-8 shadow-sm space-y-6">
            {/* Education */}
            <div className="space-y-4">
              <div className="flex items-center justify-between pb-2 border-b border-gray-100">
                <h2 className="text-lg font-bold text-gray-900 flex items-center gap-2">
                  <GraduationCap className="w-5 h-5 text-emerald-700" />
                  Education & Qualifications
                </h2>
                <button
                  type="button"
                  onClick={addEducation}
                  className="text-xs font-medium text-emerald-800 hover:text-emerald-900 flex items-center gap-1 bg-emerald-50 px-2.5 py-1.5 rounded-lg border border-emerald-200 transition"
                >
                  <Plus className="w-3.5 h-3.5" />
                  Add Education
                </button>
              </div>

              {education.length === 0 ? (
                <p className="text-xs text-gray-500 py-2 italic">
                  No education records listed yet.
                </p>
              ) : (
                <div className="space-y-3">
                  {education.map((edu, idx) => (
                    <div key={idx} className="p-3.5 rounded-xl border border-gray-200 bg-gray-50 flex items-center justify-between gap-4">
                      <div className="grid grid-cols-1 sm:grid-cols-3 gap-2 flex-1">
                        <input
                          type="text"
                          required
                          placeholder="Institution (e.g. University of Lagos)"
                          value={edu.institution}
                          onChange={(e) => updateEducation(idx, "institution", e.target.value)}
                          className="text-xs p-2 rounded-lg border border-gray-300 bg-white"
                        />
                        <input
                          type="text"
                          required
                          placeholder="Degree (e.g. B.Sc. Environmental Science)"
                          value={edu.degree}
                          onChange={(e) => updateEducation(idx, "degree", e.target.value)}
                          className="text-xs p-2 rounded-lg border border-gray-300 bg-white"
                        />
                        <input
                          type="text"
                          placeholder="Field of study or graduation year"
                          value={edu.field_of_study || ""}
                          onChange={(e) => updateEducation(idx, "field_of_study", e.target.value)}
                          className="text-xs p-2 rounded-lg border border-gray-300 bg-white"
                        />
                      </div>
                      <button
                        type="button"
                        onClick={() => removeEducation(idx)}
                        className="text-gray-400 hover:text-rose-600 transition"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>

            {/* Certifications */}
            <div className="space-y-4 pt-4 border-t border-gray-100">
              <div className="flex items-center justify-between pb-2 border-b border-gray-100">
                <h2 className="text-lg font-bold text-gray-900 flex items-center gap-2">
                  <Award className="w-5 h-5 text-emerald-700" />
                  Professional Certifications
                </h2>
                <button
                  type="button"
                  onClick={addCertification}
                  className="text-xs font-medium text-emerald-800 hover:text-emerald-900 flex items-center gap-1 bg-emerald-50 px-2.5 py-1.5 rounded-lg border border-emerald-200 transition"
                >
                  <Plus className="w-3.5 h-3.5" />
                  Add Certification
                </button>
              </div>

              {certifications.length === 0 ? (
                <p className="text-xs text-gray-500 py-2 italic">
                  No professional certifications listed yet.
                </p>
              ) : (
                <div className="space-y-3">
                  {certifications.map((cert, idx) => (
                    <div key={idx} className="p-3.5 rounded-xl border border-gray-200 bg-gray-50 flex items-center justify-between gap-4">
                      <div className="grid grid-cols-1 sm:grid-cols-3 gap-2 flex-1">
                        <input
                          type="text"
                          required
                          placeholder="Certification name (e.g. OSHA 30)"
                          value={cert.name}
                          onChange={(e) => updateCertification(idx, "name", e.target.value)}
                          className="text-xs p-2 rounded-lg border border-gray-300 bg-white"
                        />
                        <input
                          type="text"
                          required
                          placeholder="Issuing body (e.g. ISPON)"
                          value={cert.issuing_org}
                          onChange={(e) => updateCertification(idx, "issuing_org", e.target.value)}
                          className="text-xs p-2 rounded-lg border border-gray-300 bg-white"
                        />
                        <input
                          type="text"
                          placeholder="Credential ID or Year"
                          value={cert.credential_id || ""}
                          onChange={(e) => updateCertification(idx, "credential_id", e.target.value)}
                          className="text-xs p-2 rounded-lg border border-gray-300 bg-white"
                        />
                      </div>
                      <button
                        type="button"
                        onClick={() => removeCertification(idx)}
                        className="text-gray-400 hover:text-rose-600 transition"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>

          {/* Submit Action */}
          <div className="flex items-center justify-end gap-4 pt-4">
            <Link
              href="/careers"
              className="text-sm font-medium text-gray-600 hover:text-gray-900 transition"
            >
              Cancel
            </Link>
            <button
              type="submit"
              disabled={saving}
              className="px-6 py-2.5 rounded-lg bg-emerald-800 hover:bg-emerald-900 text-white font-medium text-sm transition shadow-sm flex items-center gap-2 disabled:opacity-50"
            >
              {saving ? (
                <>
                  <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
                  Saving Profile...
                </>
              ) : (
                <>
                  <Save className="w-4 h-4" />
                  Save Profile Changes
                </>
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

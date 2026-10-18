"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { User, MapPin, Shield, ArrowLeft, Check, AlertCircle } from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import { State, LGA } from "@/lib/types";
import { Alert } from "@/components/ui/Alert";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";

export default function MemberProfilePage() {
  const router = useRouter();
  const { user, isLoading, refreshUser } = useAuth();

  const [states, setStates] = useState<State[]>([]);
  const [lgas, setLgas] = useState<LGA[]>([]);

  const [displayName, setDisplayName] = useState("");
  const [homeStateId, setHomeStateId] = useState("");
  const [homeLgaId, setHomeLgaId] = useState("");
  const [address, setAddress] = useState("");
  const [bio, setBio] = useState("");
  const [isPublicBadges, setIsPublicBadges] = useState(true);

  const [saving, setSaving] = useState(false);
  const [feedback, setFeedback] = useState<{ type: "success" | "error"; message: string } | null>(null);

  useEffect(() => {
    if (!isLoading && !user) {
      router.push("/login?redirect=/member/profile");
      return;
    }

    if (user) {
      setDisplayName(user.display_name || "");
      if (user.profile) {
        setHomeStateId(user.profile.home_state_id || "");
        setHomeLgaId(user.profile.home_lga_id || "");
        setAddress(user.profile.address || "");
        setBio(user.profile.bio || "");
        setIsPublicBadges(user.profile.is_public_badges ?? true);
      }
    }
  }, [user, isLoading, router]);

  useEffect(() => {
    async function loadStates() {
      try {
        const data = await apiRequest<State[]>("/geography/states");
        setStates(data || []);
      } catch (err) {
        console.error("Failed to load states", err);
      }
    }
    loadStates();
  }, []);

  useEffect(() => {
    if (!homeStateId) {
      setLgas([]);
      return;
    }
    async function loadLgas() {
      try {
        const data = await apiRequest<LGA[]>(`/geography/states/${homeStateId}/lgas`);
        setLgas(data || []);
      } catch (err) {
        console.error("Failed to load LGAs", err);
      }
    }
    loadLgas();
  }, [homeStateId]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setFeedback(null);

    try {
      await apiRequest("/members/me", {
        method: "PUT",
        body: JSON.stringify({
          display_name: displayName,
          home_state_id: homeStateId || null,
          home_lga_id: homeLgaId || null,
          address: address || null,
          bio: bio || null,
          is_public_badges: isPublicBadges,
        }),
      });

      await refreshUser();
      setFeedback({ type: "success", message: "Profile updated successfully." });
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to update profile." });
    } finally {
      setSaving(false);
    }
  };

  if (isLoading) return <Spinner text="Loading profile..." />;
  if (!user) return null;

  return (
    <div className="max-w-3xl mx-auto px-4 sm:px-6 lg:px-8 py-12 space-y-8">
      <Link
        href="/member"
        className="inline-flex items-center text-xs font-semibold text-gray-500 hover:text-gray-900"
      >
        <ArrowLeft className="w-4 h-4 mr-1" />
        Back to Dashboard
      </Link>

      <div className="space-y-2">
        <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900">Member Profile</h1>
        <p className="text-xs text-gray-600">
          Manage your personal details, home location chapter, and badge visibility preferences.
        </p>
      </div>

      {feedback && (
        <Alert type={feedback.type}>
          {feedback.message}
        </Alert>
      )}

      {/* Account Info Read-Only Snapshot */}
      <div className="bg-gray-50 rounded-2xl border border-gray-200 p-5 space-y-3 text-xs">
        <h3 className="font-bold text-gray-900 text-sm">Account Overview</h3>
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <div>
            <span className="text-gray-500 block">Membership Reference:</span>
            <strong className="font-mono text-gray-900">{user.membership_reference}</strong>
          </div>
          <div>
            <span className="text-gray-500 block">Registered Email:</span>
            <span className="text-gray-900">{user.email || "Not set"}</span>
          </div>
          <div>
            <span className="text-gray-500 block">Movement Role:</span>
            <Badge variant="primary" size="sm">{user.primary_role.replace("_", " ")}</Badge>
          </div>
        </div>
      </div>

      {/* Profile Edit Form */}
      <form onSubmit={handleSubmit} className="bg-white p-6 sm:p-8 rounded-2xl border border-gray-200 shadow-sm space-y-5">
        <div>
          <label className="block text-xs font-semibold text-gray-700 mb-1">Display Name *</label>
          <input
            type="text"
            required
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
          />
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div>
            <label className="block text-xs font-semibold text-gray-700 mb-1">Home State / FCT</label>
            <select
              value={homeStateId}
              onChange={(e) => {
                setHomeStateId(e.target.value);
                setHomeLgaId("");
              }}
              className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:ring-emerald-700 focus:border-emerald-700 bg-white"
            >
              <option value="">Select State or Territory</option>
              {states.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name} {s.is_fct ? "(Territory)" : ""}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-xs font-semibold text-gray-700 mb-1">Home LGA / Area Council</label>
            <select
              value={homeLgaId}
              onChange={(e) => setHomeLgaId(e.target.value)}
              disabled={!homeStateId || lgas.length === 0}
              className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:ring-emerald-700 focus:border-emerald-700 bg-white disabled:bg-gray-100"
            >
              <option value="">Select Local Area</option>
              {lgas.map((l) => (
                <option key={l.id} value={l.id}>
                  {l.name}
                </option>
              ))}
            </select>
          </div>
        </div>

        <div>
          <label className="block text-xs font-semibold text-gray-700 mb-1">Residential / Area Address</label>
          <input
            type="text"
            value={address}
            onChange={(e) => setAddress(e.target.value)}
            placeholder="e.g. 14 Airport Road"
            className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
          />
        </div>

        <div>
          <label className="block text-xs font-semibold text-gray-700 mb-1">Volunteer Bio & Skills</label>
          <textarea
            rows={3}
            value={bio}
            onChange={(e) => setBio(e.target.value)}
            placeholder="Brief description of your volunteer background or community interests..."
            className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
          />
        </div>

        <div className="pt-2">
          <label className="flex items-center gap-2 cursor-pointer text-xs text-gray-700">
            <input
              type="checkbox"
              checked={isPublicBadges}
              onChange={(e) => setIsPublicBadges(e.target.checked)}
              className="rounded border-gray-300 text-emerald-800 focus:ring-emerald-700"
            />
            <span>Show my earned participation badges on my public profile</span>
          </label>
        </div>

        <button
          type="submit"
          disabled={saving}
          className="w-full py-2.5 px-4 rounded-xl text-sm font-semibold text-white bg-emerald-800 hover:bg-emerald-900 shadow-sm transition disabled:opacity-50"
        >
          {saving ? "Saving Changes..." : "Save Profile"}
        </button>
      </form>
    </div>
  );
}

"use client";

import React, { useEffect, useState } from "react";
import { Shield, Search, UserCheck, Clock, Mail } from "lucide-react";
import { apiRequest } from "@/lib/api";
import { PublicLeader } from "@/lib/types";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";

export default function LeadershipDirectoryPage() {
  const [leaders, setLeaders] = useState<PublicLeader[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [typeFilter, setTypeFilter] = useState("all");

  useEffect(() => {
    async function fetchLeaders() {
      try {
        const data = await apiRequest<PublicLeader[]>("/leadership/directory");
        setLeaders(data || []);
      } catch (err) {
        console.error("Failed to load leadership directory", err);
      } finally {
        setLoading(false);
      }
    }
    fetchLeaders();
  }, []);

  const filteredLeaders = leaders.filter((l) => {
    const matchesSearch =
      l.role_title.toLowerCase().includes(search.toLowerCase()) ||
      (l.jurisdiction_name && l.jurisdiction_name.toLowerCase().includes(search.toLowerCase())) ||
      l.display_name.toLowerCase().includes(search.toLowerCase());

    const matchesType = typeFilter === "all" || l.jurisdiction_type === typeFilter;

    return matchesSearch && matchesType;
  });

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12 space-y-8">
      <div className="space-y-3">
        <Badge variant="primary" size="md">Governance & Accountability</Badge>
        <h1 className="text-3xl font-extrabold text-gray-900 tracking-tight">
          Approved Leadership Directory
        </h1>
        <p className="text-sm text-gray-600 max-w-2xl leading-relaxed">
          Public directory of officially appointed coordinators covering National Operations, the 6 Geopolitical Zones,
          36 States, and the Federal Capital Territory. Jurisdictions without an active appointment display &ldquo;Appointment pending&rdquo;.
        </p>
      </div>

      <Alert type="info">
        <strong>Strict Leadership Policy:</strong> Coordinators possess administrative authority strictly within their
        recorded jurisdiction. Names are published only upon verified appointment records; no names are fabricated.
      </Alert>

      {/* Filters & Search */}
      <div className="flex flex-col sm:flex-row gap-4 justify-between items-stretch sm:items-center bg-white p-4 rounded-xl border border-gray-200">
        <div className="relative flex-1 max-w-md">
          <Search className="w-4 h-4 text-gray-400 absolute left-3 top-3" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search by state, title, or coordinator..."
            className="w-full pl-9 pr-4 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
          />
        </div>

        <div className="flex gap-2">
          {["all", "national", "zone", "state"].map((t) => (
            <button
              key={t}
              onClick={() => setTypeFilter(t)}
              className={`px-3 py-1.5 text-xs font-semibold rounded-lg capitalize transition ${
                typeFilter === t
                  ? "bg-emerald-900 text-white"
                  : "bg-gray-100 text-gray-700 hover:bg-gray-200"
              }`}
            >
              {t === "state" ? "States & FCT" : t}
            </button>
          ))}
        </div>
      </div>

      {loading ? (
        <Spinner text="Loading leadership directory..." />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {filteredLeaders.map((leader, idx) => (
            <div
              key={idx}
              className="bg-white rounded-xl border border-gray-200 p-5 shadow-sm space-y-4 hover:border-emerald-300 transition"
            >
              <div className="flex justify-between items-start gap-2">
                <div>
                  <h3 className="font-bold text-gray-900 text-base">{leader.role_title}</h3>
                  <span className="text-xs text-gray-500 font-medium">
                    {leader.jurisdiction_name || "National"}
                  </span>
                </div>
                <Badge
                  variant={leader.status === "active" ? "success" : "warning"}
                  size="sm"
                >
                  {leader.status}
                </Badge>
              </div>

              <div className="pt-2 border-t border-gray-100 space-y-2 text-xs">
                <div className="flex items-center gap-2 text-gray-700">
                  <UserCheck className="w-4 h-4 text-emerald-700 flex-shrink-0" />
                  <span className="font-semibold text-sm">
                    {leader.display_name}
                  </span>
                </div>

                {leader.public_contact ? (
                  <div className="flex items-center gap-2 text-gray-600">
                    <Mail className="w-4 h-4 text-gray-400 flex-shrink-0" />
                    <span>{leader.public_contact}</span>
                  </div>
                ) : (
                  <div className="text-gray-400 italic">No public office contact registered</div>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

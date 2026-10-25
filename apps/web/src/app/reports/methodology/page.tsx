"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import {
  Info,
  ShieldCheck,
  ArrowLeft,
  FileText,
  Layers,
  Database,
  Lock,
  CheckCircle2,
  Calendar,
  AlertTriangle,
} from "lucide-react";
import { apiRequest } from "@/lib/api";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";
import { Badge } from "@/components/ui/Badge";

export default function MethodologyReportPage() {
  const [methodology, setMethodology] = useState<any>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function loadMethodology() {
      try {
        const res = await apiRequest<any>("/public/reports/methodology");
        if (res) setMethodology(res);
      } catch (err) {
        console.error("Failed loading methodology", err);
      } finally {
        setLoading(false);
      }
    }
    loadMethodology();
  }, []);

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-10">
      {/* Breadcrumb */}
      <div className="flex items-center gap-2 text-xs text-gray-500 font-medium">
        <Link href="/reports" className="flex items-center gap-1 hover:text-emerald-800 transition">
          <ArrowLeft className="w-3.5 h-3.5" />
          <span>All Public Reports</span>
        </Link>
        <span>/</span>
        <span className="text-gray-900 font-bold">Data Sources & Counting Methodology</span>
      </div>

      {/* Header */}
      <div className="bg-gradient-to-r from-emerald-950 via-emerald-900 to-gray-900 text-white rounded-3xl p-6 sm:p-10 shadow-sm border border-emerald-800 space-y-3">
        <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-emerald-800/80 border border-emerald-600/50 text-emerald-300 text-xs font-semibold">
          <ShieldCheck className="w-4 h-4 text-emerald-400" />
          <span>Scientific Rigor & Governance Transparency Standards</span>
        </div>
        <h1 className="text-3xl sm:text-4xl font-extrabold tracking-tight">
          Data Sources, Aggregation Logic & Counting Rules
        </h1>
        <p className="text-sm text-emerald-100/90 max-w-3xl leading-relaxed">
          The Nigerian Ratels platform maintains strict separation between verified field achievements,
          reconciled accounting ledgers, and provisional disclosures. This document details how each metric
          is collected, verified, and published.
        </p>
      </div>

      {loading ? (
        <Spinner text="Loading methodology standards..." />
      ) : (
        <div className="space-y-8">
          {/* Core Indicator Definitions */}
          <div className="bg-white rounded-2xl border border-gray-200 p-6 sm:p-8 space-y-6 shadow-sm">
            <h2 className="text-lg font-bold text-gray-900 flex items-center gap-2">
              <Database className="w-5 h-5 text-emerald-800" />
              <span>Standardized Data Definitions</span>
            </h2>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div className="p-4 bg-gray-50 rounded-xl space-y-1.5 border border-gray-100">
                <span className="text-xs font-extrabold text-emerald-950 uppercase tracking-wider">
                  1. Completed Cleanup Activity
                </span>
                <p className="text-xs text-gray-700 leading-relaxed">
                  An environmental sanitation activity executed on the official last Saturday schedule.
                  The venue draft must be inspected by an authorized state coordinator, published for volunteer sign-ups,
                  executed, and marked completed with verified duration and disposal documentation.
                </p>
              </div>

              <div className="p-4 bg-gray-50 rounded-xl space-y-1.5 border border-gray-100">
                <span className="text-xs font-extrabold text-emerald-950 uppercase tracking-wider">
                  2. Verified Civic Participation
                </span>
                <p className="text-xs text-gray-700 leading-relaxed">
                  An attendance record supported by geolocation and photographic evidence. Must be independently reviewed
                  and approved by an authorized coordinator. The system enforces an anti-self-approval invariant:
                  a coordinator cannot verify their own attendance claim.
                </p>
              </div>

              <div className="p-4 bg-gray-50 rounded-xl space-y-1.5 border border-gray-100">
                <span className="text-xs font-extrabold text-emerald-950 uppercase tracking-wider">
                  3. Unique Participants vs Participation Claims
                </span>
                <p className="text-xs text-gray-700 leading-relaxed">
                  Participation claims represent event-level attendances. Unique participants represent the deduplicated
                  count of distinct human beings. A member attending 10 monthly cleanups generates 10 participation claims,
                  but contributes exactly 1 to the unique participant metric for that year.
                </p>
              </div>

              <div className="p-4 bg-gray-50 rounded-xl space-y-1.5 border border-gray-100">
                <span className="text-xs font-extrabold text-emerald-950 uppercase tracking-wider">
                  4. Approved Community Projects & Milestones
                </span>
                <p className="text-xs text-gray-700 leading-relaxed">
                  Community infrastructure projects must have an approved budget and a recognized community ward location.
                  Physical completion is tracked through independent inspections totaling 100%, and is decoupled from
                  financial disbursements to prevent disbursement inflation.
                </p>
              </div>

              <div className="p-4 bg-gray-50 rounded-xl space-y-1.5 border border-gray-100">
                <span className="text-xs font-extrabold text-emerald-950 uppercase tracking-wider">
                  5. Reconciled Financial Figures
                </span>
                <p className="text-xs text-gray-700 leading-relaxed">
                  All published income and expenditure amounts originate from the authoritative double-entry general ledger
                  and must reconcile against imported bank statements. Internal fund transfers are flagged as non-external
                  to prevent double-counting.
                </p>
              </div>

              <div className="p-4 bg-gray-50 rounded-xl space-y-1.5 border border-gray-100">
                <span className="text-xs font-extrabold text-emerald-950 uppercase tracking-wider">
                  6. Foundation Segregation & Provisional Status
                </span>
                <p className="text-xs text-gray-700 leading-relaxed">
                  Charitable donations to the Martinsvincent Ortse Foundation are held in dedicated restricted accounts and
                  segregated from Ratels operational expenses. The Foundation registration is labeled provisional pending final
                  CAC review.
                </p>
              </div>
            </div>
          </div>

          {/* Published Dataset Lineage Table */}
          <div className="bg-white rounded-2xl border border-gray-200 overflow-hidden shadow-sm p-6 sm:p-8 space-y-4">
            <h2 className="text-lg font-bold text-gray-900 flex items-center gap-2">
              <Layers className="w-5 h-5 text-emerald-800" />
              <span>Published Datasets & Governance Traceability</span>
            </h2>

            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead className="bg-gray-50 text-gray-600 font-semibold uppercase tracking-wider border-b border-gray-200">
                  <tr>
                    <th className="py-3 px-4">Dataset</th>
                    <th className="py-3 px-4">Source Module</th>
                    <th className="py-3 px-4">Geographic Scope</th>
                    <th className="py-3 px-4">Calculation Method</th>
                    <th className="py-3 px-4">Approval Authority</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">
                  <tr className="hover:bg-gray-50/50">
                    <td className="py-3.5 px-4 font-bold text-gray-900">National Overview</td>
                    <td className="py-3.5 px-4">Cross-cutting engine</td>
                    <td className="py-3.5 px-4 text-gray-600">Nationwide / Zone / State</td>
                    <td className="py-3.5 px-4 text-gray-600">Automated SQL rollup of verified rows</td>
                    <td className="py-3.5 px-4 font-semibold text-emerald-900">Super Admin</td>
                  </tr>
                  <tr className="hover:bg-gray-50/50">
                    <td className="py-3.5 px-4 font-bold text-gray-900">Cleanups & Waste</td>
                    <td className="py-3.5 px-4">Cleanups & Venues</td>
                    <td className="py-3.5 px-4 text-gray-600">State / LGA</td>
                    <td className="py-3.5 px-4 text-gray-600">Completed venues + scale receipts</td>
                    <td className="py-3.5 px-4 font-semibold text-emerald-900">State Coordinator / National Admin</td>
                  </tr>
                  <tr className="hover:bg-gray-50/50">
                    <td className="py-3.5 px-4 font-bold text-gray-900">Civic Participation</td>
                    <td className="py-3.5 px-4">Participation Queue</td>
                    <td className="py-3.5 px-4 text-gray-600">Nationwide / State</td>
                    <td className="py-3.5 px-4 text-gray-600">Deduplicated user claims with suppression</td>
                    <td className="py-3.5 px-4 font-semibold text-emerald-900">Reviewing Coordinator</td>
                  </tr>
                  <tr className="hover:bg-gray-50/50">
                    <td className="py-3.5 px-4 font-bold text-gray-900">Financial Disclosures</td>
                    <td className="py-3.5 px-4">General Ledger</td>
                    <td className="py-3.5 px-4 text-gray-600">Nationwide / State</td>
                    <td className="py-3.5 px-4 text-gray-600">Reconciled debits/credits in integer kobo</td>
                    <td className="py-3.5 px-4 font-semibold text-emerald-900">Super Admin (Mandatory)</td>
                  </tr>
                  <tr className="hover:bg-gray-50/50">
                    <td className="py-3.5 px-4 font-bold text-gray-900">Foundation Reports</td>
                    <td className="py-3.5 px-4">Foundation Ledger</td>
                    <td className="py-3.5 px-4 text-gray-600">Nationwide</td>
                    <td className="py-3.5 px-4 text-gray-600">Segregated charitable inflows & outreaches</td>
                    <td className="py-3.5 px-4 font-semibold text-emerald-900">Board of Trustees / Super Admin</td>
                  </tr>
                  <tr className="hover:bg-gray-50/50">
                    <td className="py-3.5 px-4 font-bold text-gray-900">Community Projects</td>
                    <td className="py-3.5 px-4">Community Projects</td>
                    <td className="py-3.5 px-4 text-gray-600">State / LGA / Ward</td>
                    <td className="py-3.5 px-4 text-gray-600">Milestone inspections + voucher spending</td>
                    <td className="py-3.5 px-4 font-semibold text-emerald-900">Project Officer & Coordinator</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          {/* Privacy & Confidentiality Policy */}
          <div className="bg-gray-50 rounded-2xl border border-gray-200 p-6 sm:p-8 space-y-4">
            <h2 className="text-base font-bold text-gray-900 flex items-center gap-2">
              <Lock className="w-5 h-5 text-emerald-800" />
              <span>Strict Privacy & Confidentiality Invariants</span>
            </h2>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs text-gray-700">
              <div className="p-4 bg-white rounded-xl border border-gray-200 space-y-1">
                <span className="font-bold text-gray-900">Zero Safeguarding Leakage</span>
                <p>
                  Safeguarding incident reports, whistleblower tracking secrets, and confidential investigation notes
                  are completely quarantined in isolated tables with AES-256-GCM encryption. The reporting engine has zero
                  read permissions to safeguarding schemas.
                </p>
              </div>

              <div className="p-4 bg-white rounded-xl border border-gray-200 space-y-1">
                <span className="font-bold text-gray-900">Personal Information Protection</span>
                <p>
                  No member emails, phone numbers, home addresses, dates of birth, or individual resumes are ever exposed
                  in public API responses or downloadable PDF/CSV reports.
                </p>
              </div>

              <div className="p-4 bg-white rounded-xl border border-gray-200 space-y-1">
                <span className="font-bold text-gray-900">Small-Cell Suppression</span>
                <p>
                  Any subgroup count below 5 participants is automatically masked to prevent re-identification through
                  differencing attacks across geographic filters.
                </p>
              </div>

              <div className="p-4 bg-white rounded-xl border border-gray-200 space-y-1">
                <span className="font-bold text-gray-900">Banking Security & Redaction</span>
                <p>
                  Internal bank accounts, vendor payment details, and Paystack API credentials are never published.
                  Only approved category aggregates are exposed.
                </p>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

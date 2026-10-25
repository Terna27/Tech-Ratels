import React from "react";
import Link from "next/link";
import { Shield } from "lucide-react";

export function Footer() {
  return (
    <footer className="bg-gray-900 text-gray-300 border-t border-gray-800 text-sm mt-auto">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10">
        <div className="grid grid-cols-1 md:grid-cols-4 gap-8">
          {/* Movement identity */}
          <div className="md:col-span-2">
            <div className="flex items-center gap-2 mb-3">
              <div className="w-8 h-8 rounded-lg bg-emerald-800 flex items-center justify-center text-white">
                <Shield className="w-4 h-4 text-emerald-200" aria-hidden="true" />
              </div>
              <span className="font-bold text-white text-base">Ratels Worldwide</span>
            </div>
            <p className="text-gray-400 text-xs leading-relaxed max-w-md mb-4">
              Building a coordinated movement for environmental sanitation, community
              service, humanitarian support, and social transparency across the 36 States, the FCT, and global diaspora networks.
            </p>
            <div className="p-3 bg-gray-800/80 rounded-lg text-xs text-gray-400 border border-gray-700 leading-normal">
              <strong className="text-gray-200 block mb-1">Governance Baseline & Transparency:</strong>
              This platform implementation is based on the Ratels Worldwide blueprint. Role titles,
              certified metrics, and provisional Foundation operations follow strict governance workflows, verified registration, and leadership authorization.
            </div>
          </div>

          {/* Quick Navigation */}
          <div>
            <h2 className="text-white font-semibold text-xs uppercase tracking-wider mb-3">
              Platform Areas
            </h2>
            <ul className="space-y-2 text-xs" role="list">
              <li>
                <Link href="/activities" className="hover:text-emerald-400 transition focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-emerald-400 rounded">
                  Monthly Cleanups
                </Link>
              </li>
              <li>
                <Link href="/leadership" className="hover:text-emerald-400 transition focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-emerald-400 rounded">
                  Leadership Directory
                </Link>
              </li>
              <li>
                <Link href="/foundation" className="hover:text-emerald-400 transition focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-emerald-400 rounded">
                  Martins Vincent Otse Foundation
                </Link>
              </li>
              <li>
                <Link href="/marketplace" className="hover:text-emerald-400 transition focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-emerald-400 rounded">
                  Ratels Marketplace
                </Link>
              </li>
              <li>
                <Link href="/careers" className="hover:text-emerald-400 transition focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-emerald-400 rounded">
                  Ratels Careers
                </Link>
              </li>
              <li>
                <Link href="/reports" className="hover:text-emerald-400 transition focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-emerald-400 rounded">
                  Public Reports & Disclosures
                </Link>
              </li>
              <li>
                <Link href="/about" className="hover:text-emerald-400 transition focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-emerald-400 rounded">
                  About the Movement
                </Link>
              </li>
              <li>
                <Link href="/report-incident" className="text-amber-400 hover:text-amber-300 font-semibold transition flex items-center gap-1 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-amber-400 rounded">
                  Report Incident (Confidential)
                </Link>
              </li>
            </ul>
          </div>

          {/* Platform Standards */}
          <div>
            <h2 className="text-white font-semibold text-xs uppercase tracking-wider mb-3">
              Standards & Compliance
            </h2>
            <ul className="space-y-2 text-xs text-gray-400" role="list">
              <li className="flex items-center gap-1.5">
                <span>Nigeria Data Protection Act (NDPA)</span>
                <span className="text-[10px] px-1.5 py-0.5 bg-emerald-950 text-emerald-300 border border-emerald-800 rounded">Active</span>
              </li>
              <li className="flex items-center gap-1.5">
                <span>WCAG 2.2 AA Accessibility</span>
                <span className="text-[10px] px-1.5 py-0.5 bg-emerald-950 text-emerald-300 border border-emerald-800 rounded">Standard</span>
              </li>
              <li className="flex items-center gap-1.5">
                <span>Segregated Charitable Ledgers</span>
                <span className="text-[10px] px-1.5 py-0.5 bg-emerald-950 text-emerald-300 border border-emerald-800 rounded">Enforced</span>
              </li>
              <li className="flex items-center gap-1.5">
                <span>End-to-End Incident Cryptography</span>
                <span className="text-[10px] px-1.5 py-0.5 bg-emerald-950 text-emerald-300 border border-emerald-800 rounded">AES-256</span>
              </li>
              <li className="flex items-center gap-1.5">
                <span>Public Transparency Portal</span>
                <span className="text-[10px] px-1.5 py-0.5 bg-emerald-950 text-emerald-300 border border-emerald-800 rounded">Certified</span>
              </li>
            </ul>
          </div>
        </div>

        <div className="mt-8 pt-6 border-t border-gray-800 flex flex-col sm:flex-row justify-between items-center text-xs text-gray-500 gap-3">
          <div>
            © 2026 Ratels Worldwide. All rights reserved.
          </div>
          <div className="flex flex-wrap items-center gap-4">
            <span>Africa/Lagos (UTC+1)</span>
            <span>•</span>
            <span>Nigeria Data Protection Act 2023 Principles</span>
            <span>•</span>
            <Link href="/report-incident/status" className="hover:text-gray-300 transition">
              Incident Tracking
            </Link>
          </div>
        </div>
      </div>
    </footer>
  );
}

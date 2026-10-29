"use client";
import {useEffect,useState,use} from "react";
import {apiRequest} from "@/lib/api";
import {PublicMembership,membershipLabel} from "@/lib/membership";
export default function VerifyMembershipPage({params}:{params:Promise<{token:string}>}){
 const {token}=use(params);const [member,setMember]=useState<PublicMembership|null>(null),[error,setError]=useState(""),[loading,setLoading]=useState(true);
 useEffect(()=>{let live=true;setLoading(true);setMember(null);setError("");apiRequest<PublicMembership>(`/members/verify/${encodeURIComponent(token)}`).then(m=>{if(live)setMember(m);}).catch(()=>{if(live)setError("This membership link could not be verified. Check the link or try again later.");}).finally(()=>{if(live)setLoading(false);});return()=>{live=false};},[token]);
 return <main className="max-w-xl mx-auto p-5 sm:p-10"><section className="bg-white rounded-2xl border p-6 space-y-4"><h1 className="text-2xl font-bold">Ratels membership verification</h1>{loading&&<p role="status">Checking current membership…</p>}{error&&<p role="alert" className="text-red-800">{error}</p>}{member&&<><p className={member.status==="active"?"text-emerald-800 font-bold":"text-red-800 font-bold"}>{member.status==="active"?"Active membership confirmed":"This membership is not active"}</p>{member.display_name&&<h2 className="text-xl font-bold break-words">{member.display_name}</h2>}<p className="font-mono break-all">{member.membership_reference}</p><p>Status: {membershipLabel(member.status)}</p><p className="text-sm text-gray-600">This confirms membership status only. It does not confirm an administrative role or authorize financial transactions.</p></>}</section></main>;
}

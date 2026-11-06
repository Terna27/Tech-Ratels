"use client";
import React, { useEffect, useState } from "react";
import Link from "next/link";
import { apiRequest } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";

type Report = {id:string;review_id:string;product_id:string;comment:string;rating:number;review_status:string;reason:string;created_at:string};

export default function MarketplaceReviewsAdmin() {
 const {isSuperAdmin,isLoading}=useAuth();
 const [reports,setReports]=useState<Report[]>([]);
 const [error,setError]=useState("");
 const [notice,setNotice]=useState("");
 const [loading,setLoading]=useState(true);
 const load=async()=>{
  try{setReports(await apiRequest<Report[]>("/marketplace/admin/reviews/reports"));}
  catch(e:any){setError(e?.message||"Unable to load reports");}
  finally{setLoading(false);}
 };
 useEffect(()=>{if(!isLoading&&isSuperAdmin){void load();}else if(!isLoading){setLoading(false);}},[isLoading,isSuperAdmin]);
 const moderate=async(id:string,status:"hidden"|"published")=>{
  setError("");setNotice("");
  try{
   await apiRequest("/marketplace/admin/reviews/"+id+"/moderate",{method:"PATCH",body:JSON.stringify({status})});
   setNotice("Review status updated.");
   await load();
  }catch(e:any){setError(e?.message||"Moderation failed");}
 };
 if(isLoading||loading)return <div className="p-8 text-gray-500">Loading...</div>;
 if(!isSuperAdmin)return <div className="p-8 text-red-700">Super Admin access required.</div>;
 return <main className="max-w-5xl mx-auto p-6 sm:p-10 space-y-6">
  <div><Link href="/admin/marketplace" className="text-sm text-emerald-800">Marketplace administration</Link><h1 className="text-2xl font-bold mt-2">Customer review moderation</h1>
  <p className="text-sm text-gray-600">Investigate reported reviews. Only Super Admin may hide or restore customer reviews.</p></div>
  {error&&<p role="alert" className="text-sm text-red-700">{error}</p>}
  {notice&&<p role="status" className="text-sm text-emerald-700">{notice}</p>}
  {reports.length===0?<p className="text-gray-500">No reported reviews.</p>:<div className="space-y-4">
   {reports.map(item=><article key={item.id} className="rounded-xl bg-white border p-5 space-y-3">
    <div className="flex flex-wrap justify-between gap-2"><strong>Customer review report</strong><span className="text-xs text-gray-500">{new Date(item.created_at).toLocaleString()}</span></div>
    <p className="text-sm"><strong>Reason:</strong> {item.reason}</p>
    <p className="text-sm text-gray-700 whitespace-pre-wrap">{item.comment}</p>
    <p className="text-sm">Rating: {item.rating}/5 | Status: {item.review_status}</p>
    <Link href={"/marketplace/products/"+item.product_id} className="text-sm underline text-emerald-800">View product</Link>
    <div className="flex gap-3">
     <button onClick={()=>{void moderate(item.review_id,"hidden")}} type="button" className="bg-red-800 text-white text-sm rounded-lg px-4 py-2">Hide review</button>
     <button onClick={()=>{void moderate(item.review_id,"published")}} type="button" className="bg-emerald-900 text-white text-sm rounded-lg px-4 py-2">Restore review</button>
    </div>
   </article>)}
  </div>}
 </main>;
}

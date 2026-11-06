"use client";
import React, { useCallback, useEffect, useState } from "react";
import { Star, Camera } from "lucide-react";
import { apiRequest } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";

type Review = {
 id:string; buyer_name:string; rating:number; comment:string;
 images:string[]; vendor_response?:string|null; created_at:string; is_mine?:boolean;
};
type ReviewResult = {reviews:Review[];count:number;average_rating:number;page?:number;page_size?:number;has_more?:boolean};

export default function ProductReviews({productId,isVendor=false}:{productId:string;isVendor?:boolean}) {
 const {user}=useAuth();
 const [data,setData]=useState<ReviewResult>({reviews:[],count:0,average_rating:0});
 const [loading,setLoading]=useState(true);
 const [rating,setRating]=useState(5);
 const [comment,setComment]=useState("");
 const [photos,setPhotos]=useState<string[]>([]);
 const [working,setWorking]=useState(false);
 const [error,setError]=useState("");
 const [message,setMessage]=useState("");
 const [editingId,setEditingId]=useState<string|null>(null);
 const [sort,setSort]=useState("newest");
 const [page,setPage]=useState(1);
 const reload=useCallback(async()=>{
  try{setData(await apiRequest<ReviewResult>("/marketplace/products/"+productId+"/reviews?sort="+encodeURIComponent(sort)+"&page="+page));}
  catch(e:any){setError(e?.message||"Could not load reviews");}
  finally{setLoading(false);}
 },[productId,sort,page]);
 useEffect(()=>{void reload();},[reload]);
 const upload=async(files:FileList|null)=>{
  if(!files||files.length===0)return;
  setError("");
  if(files.length+photos.length>5){setError("Maximum five images per review");return;}
  setWorking(true);
  try{
   const urls:string[]=[];
   for(const file of Array.from(files)){
    if(!["image/jpeg","image/png","image/webp"].includes(file.type)||file.size>5*1024*1024)
     throw new Error("Only JPG, PNG or WebP up to 5 MB is accepted");
    const form=new FormData();
    form.append("file",file);
    const response=await apiRequest<{public_url:string}>("/marketplace/products/"+productId+"/reviews/images",{method:"POST",body:form});
    urls.push(response.public_url);
   }
   setPhotos(prev=>[...prev,...urls]);
  }catch(e:any){setError(e?.message||"Image upload failed");}
  finally{setWorking(false);}
 };
 const submit=async(event:React.FormEvent)=>{
  event.preventDefault();setWorking(true);setError("");setMessage("");
  try{
   await apiRequest(editingId ? "/marketplace/reviews/"+editingId : "/marketplace/products/"+productId+"/reviews",{method:editingId?"PUT":"POST",body:JSON.stringify({rating,comment,images:photos})});
   setComment("");setPhotos([]);setEditingId(null);setMessage("Your verified purchase review was saved.");
   await reload();
  }catch(e:any){setError(e?.message||"Review submission failed");}
  finally{setWorking(false);}
 };
 const reply=async(id:string)=>{
  const response=window.prompt("Write a helpful vendor response (maximum 2000 characters)");
  if(!response)return;
  try{await apiRequest("/marketplace/reviews/"+id+"/reply",{method:"POST",body:JSON.stringify({reply:response})});setMessage("Vendor response posted.");await reload();}
  catch(e:any){setError(e?.message||"Could not post vendor response");}
 };
 const report=async(id:string)=>{
  const reason=window.prompt("Reason for reporting this review (minimum 5 characters)");
  if(!reason)return;
  try{await apiRequest("/marketplace/reviews/"+id+"/report",{method:"POST",body:JSON.stringify({reason})});setMessage("Report submitted.");}
  catch(e:any){setError(e?.message||"Report failed");}
 };
 return <section className="bg-white rounded-3xl border border-gray-200 p-6 sm:p-8 space-y-6" aria-label="Customer reviews">
  <div className="flex flex-wrap justify-between items-center gap-3">
   <div><h2 className="text-xl font-bold">Customer reviews</h2><p className="text-sm text-gray-500">Reviews are limited to completed purchases.</p></div>
   <div className="flex gap-2 items-center"><Star className="w-5 h-5 fill-amber-400 text-amber-500"/><strong>{data.average_rating.toFixed(1)}</strong><span className="text-sm text-gray-500">({data.count} reviews)</span></div>
  </div>
  <div className="flex flex-wrap items-center gap-3">
    <label htmlFor="review-sort" className="text-sm font-medium text-gray-700">Sort reviews</label>
    <select id="review-sort" value={sort} onChange={e=>{setSort(e.target.value);setPage(1);}}
      className="rounded-lg border px-3 py-2 text-sm">
      <option value="newest">Newest first</option>
      <option value="oldest">Oldest first</option>
      <option value="highest">Highest rated</option>
      <option value="lowest">Lowest rated</option>
      <option value="with_photos">Photo reviews first</option>
    </select>
  </div>
  {error&&<p role="alert" className="text-sm text-red-700">{error}</p>}
  {message&&<p role="status" className="text-sm text-emerald-800">{message}</p>}
  {user&&<form onSubmit={submit} className="p-5 bg-gray-50 border rounded-2xl space-y-3">
   <h3 className="font-semibold">Review your purchase</h3>
   <p className="text-xs text-gray-600">Only buyers with paid, delivered orders can post one review per product.</p>
   <div className="flex gap-2" aria-label="Rating">
    {[1,2,3,4,5].map(n=><button type="button" key={n} aria-label={n+" stars"} aria-pressed={n===rating} onClick={()=>setRating(n)}><Star className={"w-6 h-6 "+(n<=rating?"fill-amber-400 text-amber-500":"text-gray-300")}/></button>)}
   </div>
   <textarea required minLength={10} maxLength={2000} value={comment} onChange={e=>setComment(e.target.value)}
    placeholder="Describe quality, packaging and your experience" rows={4} className="w-full p-3 border rounded-xl text-sm"/>
   <label className="inline-flex gap-2 items-center cursor-pointer text-sm"><Camera className="w-4 h-4"/> Add photos (up to five)
    <input className="sr-only" type="file" accept="image/jpeg,image/png,image/webp" multiple disabled={working}
     onChange={e=>{void upload(e.target.files);e.target.value="";}}/>
   </label>
   {photos.length>0&&<div className="flex flex-wrap gap-2">{photos.map((src,i)=><div key={src} className="relative"><img src={src} alt={"Uploaded photo "+(i+1)} className="w-20 h-20 object-cover rounded-lg"/><button type="button" onClick={()=>setPhotos(v=>v.filter((_,j)=>j!==i))} aria-label="Remove photo" className="absolute top-0 right-0 bg-black text-white px-1">×</button></div>)}</div>}
   <div><button type="submit" disabled={working||comment.trim().length<10} className="bg-emerald-900 text-white px-5 py-2 rounded-xl text-sm font-semibold disabled:opacity-50">{working?"Processing...":editingId?"Save edited review":"Post review"}</button></div>
  </form>}
  {!user&&<p className="text-sm text-gray-600">Sign in to review a product you purchased and received.</p>}
  {loading?<p className="text-sm text-gray-500">Loading reviews...</p>:data.reviews.length===0?<p className="text-sm text-gray-500">No reviews yet.</p>:
   <div className="space-y-5">{data.reviews.map(review=><article key={review.id} className="border-t pt-5 space-y-2">
    <div className="flex justify-between gap-3"><strong className="text-sm">{review.buyer_name}</strong><div className="flex gap-3">{review.is_mine&&<button type="button" onClick={()=>{setEditingId(review.id);setRating(review.rating);setComment(review.comment);setPhotos(review.images||[]);}} className="text-xs text-emerald-800">Edit my review</button>}{isVendor&&<button type="button" onClick={()=>{void reply(review.id)}} className="text-xs text-emerald-800">Reply as vendor</button>}{user&&<button type="button" onClick={()=>{void report(review.id)}} className="text-xs text-gray-500 hover:text-red-700">Report review</button>}</div></div>
    <div className="flex gap-2 items-center"><span className="text-amber-500">{"★".repeat(review.rating)}{"☆".repeat(5-review.rating)}</span><span className="text-xs text-emerald-800">Verified purchase</span><span className="text-xs text-gray-500">{new Date(review.created_at).toLocaleDateString()}</span></div>
    <p className="text-sm whitespace-pre-wrap break-words">{review.comment}</p>
    {review.images?.length>0&&<div className="flex flex-wrap gap-2">{review.images.map((src,i)=><a href={src} key={src} target="_blank" rel="noopener noreferrer"><img className="h-24 w-24 object-cover rounded-lg border" src={src} alt={"Customer photo "+(i+1)}/></a>)}</div>}
    {review.vendor_response&&<div className="p-3 bg-emerald-50 rounded-xl text-sm"><strong>Vendor response</strong><p className="whitespace-pre-wrap">{review.vendor_response}</p></div>}
   </article>)}</div>}
  {data.count>20&&<nav aria-label="Review pages" className="flex items-center justify-between gap-3 border-t pt-4">
     <button type="button" disabled={page===1} onClick={()=>setPage(p=>Math.max(1,p-1))}
       className="text-sm border rounded-lg px-4 py-2 disabled:opacity-40">Previous</button>
     <span className="text-sm text-gray-600">Page {page} of {Math.max(1,Math.ceil(data.count/20))}</span>
     <button type="button" disabled={!data.has_more} onClick={()=>setPage(p=>p+1)}
       className="text-sm border rounded-lg px-4 py-2 disabled:opacity-40">Next</button>
  </nav>}
 </section>;
}

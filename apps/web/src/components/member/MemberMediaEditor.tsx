"use client";

import React, { useEffect, useState } from "react";
import { Camera, ImagePlus, Trash2, RefreshCw, Check, Move } from "lucide-react";
import { apiRequest } from "@/lib/api";

type MediaSettings = {
 avatar_url?: string; has_cover: boolean;
 avatar_x: number; avatar_y: number; avatar_zoom: number;
 cover_x: number; cover_y: number; cover_zoom: number;
};
type MediaKind = "avatar" | "cover";
type Position = {x:number;y:number;zoom:number};
const DEFAULT:MediaSettings={has_cover:false,avatar_x:50,avatar_y:50,avatar_zoom:100,cover_x:50,cover_y:50,cover_zoom:100};

export default function MemberMediaEditor({name,avatar,reference,location,bio,onChange}:{name:string;avatar?:string;reference?:string;location?:string;bio?:string;onChange:()=>Promise<void>}) {
 const [media,setMedia]=useState<MediaSettings>(DEFAULT);
 const [loading,setLoading]=useState(true);
 const [working,setWorking]=useState<MediaKind|null>(null);
 const [progress,setProgress]=useState<number|null>(null);
 const [error,setError]=useState("");
 const [message,setMessage]=useState("");
 const [coverVersion,setCoverVersion]=useState(0);
 const [selected,setSelected]=useState<{kind:MediaKind;file:File;url:string}|null>(null);
 const [positions,setPositions]=useState<Record<MediaKind,Position>>({avatar:{x:50,y:50,zoom:100},cover:{x:50,y:50,zoom:100}});

 useEffect(()=>{let active=true;apiRequest<MediaSettings>("/members/me/media").then(result=>{
  if(!active)return;
  setMedia(result);
  setPositions({avatar:{x:result.avatar_x,y:result.avatar_y,zoom:result.avatar_zoom},cover:{x:result.cover_x,y:result.cover_y,zoom:result.cover_zoom}});
 }).catch(e=>{if(active)setError(e.message||"Unable to load media settings");}).finally(()=>{if(active)setLoading(false);});
 return ()=>{active=false;};},[]);
 useEffect(()=>()=>{if(selected)URL.revokeObjectURL(selected.url);},[selected]);

 const imageURL=(kind:MediaKind)=>{
  if(selected?.kind===kind)return selected.url;
  if(kind==="avatar")return media.avatar_url||avatar||"";
  return media.has_cover?"/api/v1/members/me/media/cover?v="+coverVersion:"";
 };
 const changeFile=(kind:MediaKind,file?:File)=>{
  if(!file)return;
  setError("");setMessage("");
  if(!["image/jpeg","image/png","image/webp"].includes(file.type)||file.size>5*1024*1024){
   setError("Choose a JPEG, PNG or WebP image of at most 5 MB.");return;
  }
  setSelected({kind,file,url:URL.createObjectURL(file)});
  setPositions(prev=>({...prev,[kind]:{x:50,y:50,zoom:100}}));
 };
 const changePosition=(kind:MediaKind,which:keyof Position,value:number)=>{
  setPositions(prev=>({...prev,[kind]:{...prev[kind],[which]:value}}));
 };
 const cropAndUpload=async(kind:MediaKind)=>{
  if(selected?.kind!==kind)return;
  setWorking(kind);setProgress(0);setError("");setMessage("");
  try{
   const bitmap=await createImageBitmap(selected.file);
   try{
    const width=kind==="avatar"?600:1440, height=kind==="avatar"?600:480;
    const canvas=document.createElement("canvas");canvas.width=width;canvas.height=height;
    const ctx=canvas.getContext("2d");if(!ctx)throw new Error("Image editor unavailable");
    const position=positions[kind];
    const scale=Math.max(width/bitmap.width,height/bitmap.height)*position.zoom/100;
    const scaledW=bitmap.width*scale,scaledH=bitmap.height*scale;
    ctx.drawImage(bitmap,-(scaledW-width)*position.x/100,-(scaledH-height)*position.y/100,scaledW,scaledH);
    const result=await new Promise<Blob>((resolve,reject)=>canvas.toBlob(blob=>blob?resolve(blob):reject(new Error("Could not crop image")),"image/jpeg",0.88));
    if(result.size>5*1024*1024)throw new Error("Cropped image exceeds the 5 MB limit");
    const upload=new FormData();
    upload.append("file",new File([result],kind+".jpg",{type:"image/jpeg"}));
    const updated=await apiRequest<MediaSettings>("/members/me/media/"+kind,{method:"POST",body:upload,onUploadProgress:setProgress});
    setMedia(updated);
    setPositions(prev=>({...prev,[kind]:{x:50,y:50,zoom:100}}));
    setSelected(null);
    setCoverVersion(v=>v+1);
    await onChange();
    setMessage(kind==="avatar"?"Your profile photograph and membership card photo have been updated.":"Your cover photograph has been updated.");
   }finally{bitmap.close();}
  }catch(err:any){setError(err?.message||"Could not upload photograph");}
  finally{setWorking(null);setProgress(null);}
 };
 const persistPosition=async(kind:MediaKind)=>{
  if(!imageURL(kind))return;
  setWorking(kind);setError("");setMessage("");
  try{
   const updated=await apiRequest<MediaSettings>("/members/me/media/"+kind+"/position",{method:"PUT",body:JSON.stringify(positions[kind])});
   setMedia(updated);setMessage("Image position saved.");
  }catch(err:any){setError(err?.message||"Could not save position");}
  finally{setWorking(null);setProgress(null);}
 };
 const remove=async(kind:MediaKind)=>{
  if(!window.confirm("Remove your "+(kind==="cover"?"cover":"profile")+" photo?"))return;
  setWorking(kind);setError("");setMessage("");
  try{
   await apiRequest("/members/me/media/"+kind,{method:"DELETE"});
   const updated=await apiRequest<MediaSettings>("/members/me/media");
   setMedia(updated);
   setSelected(null);
   setCoverVersion(v=>v+1);
   setPositions(prev=>({...prev,[kind]:{x:50,y:50,zoom:100}}));
   await onChange();
   setMessage(kind==="avatar"?"Your profile photograph was removed.":"Your cover photo was removed.");
  }catch(err:any){setError(err?.message||"Could not remove photo");}
  finally{setWorking(null);setProgress(null);}
 };
 const renderImage=(kind:MediaKind)=>{
  const src=imageURL(kind),position=positions[kind];
  return src?<img src={src} alt={kind==="avatar"?"Your profile picture":"Your cover picture"}
   className="w-full h-full object-cover" style={{objectPosition:`${position.x}% ${position.y}%`,transform:`scale(${position.zoom/100})`,transformOrigin:`${position.x}% ${position.y}%`}}/>
   :kind==="avatar"?<span className="text-4xl font-bold text-white">{name.trim().split(/\s+/).slice(0,2).map(s=>s[0]||"").join("").toUpperCase()||"R"}</span>
   :<div className="w-full h-full bg-gradient-to-r from-slate-200 via-slate-100 to-slate-300"/>;
 };
 const renderControls=(kind:MediaKind)=>(
  <div className="rounded-xl bg-gray-50 border p-4 space-y-3">
   <div className="flex flex-wrap gap-3 items-center">
    <label htmlFor={"member-photo-"+kind} className="inline-flex items-center gap-2 cursor-pointer focus-within:ring-2 focus-within:ring-emerald-700 bg-white border rounded-lg px-3 py-2 text-sm font-semibold">
     {kind==="avatar"?<Camera size={16}/>:<ImagePlus size={16}/>}
     {kind==="avatar"?"Choose profile photo":"Choose cover photo"}
     <input id={"member-photo-"+kind} aria-label={kind==="avatar"?"Choose profile photo":"Choose cover photo"} type="file" className="sr-only" accept="image/jpeg,image/png,image/webp"
      disabled={Boolean(working)} onChange={e=>{changeFile(kind,e.target.files?.[0]);e.target.value="";}}/>
    </label>
    {imageURL(kind)&&<button type="button" onClick={()=>{void remove(kind);}} disabled={Boolean(working)}
      className="inline-flex gap-2 items-center text-red-700 text-sm"><Trash2 size={15}/> Remove</button>}
   </div>
   {(imageURL(kind)||selected?.kind===kind)&&<fieldset disabled={Boolean(working)} className="grid gap-2 sm:grid-cols-3">
    {(["x","y","zoom"] as const).map(field=><label key={field} className="text-xs text-gray-600">
     {field==="x"?"Horizontal":field==="y"?"Vertical":"Zoom"}: {positions[kind][field]}{field==="zoom"?"%":""}
     <input aria-label={kind+" "+field} type="range" min={field==="zoom"?100:0} max={field==="zoom"?200:100}
      value={positions[kind][field]} onChange={e=>changePosition(kind,field,Number(e.target.value))}
      className="w-full accent-emerald-700 mt-1"/>
    </label>)}
   </fieldset>}
   {selected?.kind===kind&&<button type="button" disabled={Boolean(working)} className="text-sm underline" onClick={()=>{setSelected(null);setPositions(prev=>({...prev,[kind]:{x:media[`${kind}_x`],y:media[`${kind}_y`],zoom:media[`${kind}_zoom`]}}));}}>Cancel selection</button>}
   {selected?.kind===kind?
    <button type="button" disabled={Boolean(working)} onClick={()=>{void cropAndUpload(kind);}}
     className="inline-flex items-center gap-2 bg-emerald-800 text-white rounded-lg px-4 py-2 text-sm font-semibold disabled:opacity-50">
     {working===kind?<RefreshCw size={15} className="animate-spin"/>:<Check size={15}/>} Crop & upload</button>:
    imageURL(kind)&&<button type="button" disabled={Boolean(working)} onClick={()=>{void persistPosition(kind);}}
     className="inline-flex items-center gap-2 border rounded-lg px-4 py-2 text-sm font-semibold disabled:opacity-50"><Move size={15}/> Save positioning</button>}
  </div>
 );
 return <section aria-label="Profile and cover photographs" className="space-y-4">
  <div className="overflow-hidden rounded-2xl border bg-white shadow-sm">
   <div className="aspect-[3/1] bg-emerald-900 relative overflow-hidden">{renderImage("cover")}<label htmlFor="member-photo-cover" className="absolute bottom-3 right-3 cursor-pointer rounded-full bg-white p-2 shadow" aria-label="Edit cover photograph"><Camera size={20}/></label></div>
   <div className="px-5 sm:px-8 pb-6">
    <div className="-mt-12 sm:-mt-16 relative w-24 h-24 sm:w-32 sm:h-32 rounded-full overflow-hidden border-4 border-white bg-emerald-800 flex items-center justify-center shadow-sm">
     {renderImage("avatar")}
    </div>
    <label htmlFor="member-photo-avatar" className="inline-flex items-center gap-1 mt-2 text-sm text-emerald-800 cursor-pointer"><Camera size={16}/> Edit profile photograph</label>
    <h2 className="mt-3 text-2xl font-bold text-gray-900">{name||"Ratels Member"}</h2>
    {reference&&<p className="font-mono text-sm text-gray-600">{reference}</p>}
    {location&&<p className="text-sm text-gray-600 mt-1">{location}</p>}
    {bio&&<p className="text-sm text-gray-700 mt-3 whitespace-pre-wrap break-words">{bio}</p>}
    <p className="text-xs text-gray-500 mt-3">Your personal photographs are not included in public membership QR verification.</p>
   </div>
  </div>
  {loading?<p className="text-sm text-gray-500">Loading photo settings…</p>:<>
   <div className="grid sm:grid-cols-2 gap-4">
    <div><h3 className="font-semibold mb-2 text-sm">Profile picture</h3>{renderControls("avatar")}</div>
    <div><h3 className="font-semibold mb-2 text-sm">Cover picture</h3>{renderControls("cover")}</div>
   </div>
   {progress!==null&&<div role="status" aria-live="polite"><label htmlFor="photo-upload-progress" className="text-sm">{progress===100?"Processing photograph…":`Uploading photograph: ${progress}%`}</label><progress id="photo-upload-progress" value={progress} max={100} className="block w-full"/></div>}
   {error&&<p role="alert" className="text-red-700 text-sm">{error}</p>}
   {message&&<p role="status" className="text-emerald-800 text-sm">{message}</p>}
   <p className="text-xs text-gray-500">Use JPG, PNG or WebP (maximum 5 MB). Photos are cropped before upload. Profile photo updates also appear on your membership card. Cover photos are only served through your authenticated profile endpoint.</p>
  </>}
 </section>;
}

export type SocialMember={reference:string;name:string;avatar_url?:string;bio:string;location:string;has_cover:boolean;avatar_x:number;avatar_y:number;avatar_zoom:number;cover_x:number;cover_y:number;cover_zoom:number;following:boolean;friendship:'none'|'outgoing'|'incoming'|'friends';followers:number;badges:string[]};
export type SocialPost={id:string;author_reference:string;author_name:string;avatar_url?:string;body:string;visibility:'members'|'friends';created_at:string;likes:number;liked:boolean;own:boolean};
export type SocialPreferences={discoverable:boolean;share_cover:boolean};
export const memberPath=(reference:string)=>'/member/people/'+encodeURIComponent(reference);
export const relationshipPath=(reference:string)=>'/network/members/'+encodeURIComponent(reference);
export function postPayload(body:string,visibility:string){
 const text=body.trim();if(!text||[...text].length>3000)throw new Error('Write a post of 1–3000 characters.');
 if(!['members','friends'].includes(visibility))throw new Error('Choose who can see your post.');
 return {body:text,visibility};
}
export function friendActions(state:SocialMember['friendship']){
 if(state==='incoming')return [{label:'Accept request',action:'accept',method:'POST'},{label:'Decline request',action:'decline',method:'POST'}];
 if(state==='outgoing')return [{label:'Cancel request',action:'',method:'DELETE'}];
 if(state==='friends')return [{label:'Remove friend',action:'',method:'DELETE'}];
 return [{label:'Add friend',action:'',method:'POST'}];
}

export type MembershipCard = { display_name:string; membership_reference:string; status:string; photo_url?:string; photo_x?:number; photo_y?:number; photo_zoom?:number; verification_path:string; qr_image_data:string };
export type PublicMembership = { display_name?:string; membership_reference:string; status:string };
export function membershipLabel(status:string):string {
 return ({active:"Active",pending_verification:"Verification pending",inactive:"Inactive",suspended:"Suspended",revoked:"Revoked",closed:"Closed"} as Record<string,string>)[status] || "Not active";
}
export function profilePayload(name:string,state:string,lga:string,address:string,bio:string,_photo:string,publicBadges:boolean){
 if(!name.trim()||name.trim().length>100)throw new Error("Enter a name of at most 100 characters.");
 if(lga&&!state)throw new Error("Choose a state for your LGA.");
 if(address.length>255||bio.length>2000)throw new Error("Address or biography is too long.");
 return {display_name:name.trim(),home_state_id:state,home_lga_id:lga,address,bio,is_public_badges:publicBadges};
}

import test from 'node:test';
import assert from 'node:assert/strict';
import {profilePayload,membershipLabel} from '../src/lib/membership.ts';
test('profile payload validates fields and includes only self-editable properties',()=>{
 const p=profilePayload(' Jane Member ','la','ikeja','Private address','Volunteer','https://example.test/photo.jpg',false);
 assert.equal(p.display_name,'Jane Member');assert.equal(p.is_public_badges,false);assert.deepEqual(Object.keys(p).sort(),['display_name','home_state_id','home_lga_id','address','bio','avatar_url','is_public_badges'].sort());assert.equal(p.primary_role,undefined);assert.equal(p.membership_reference,undefined);
 for(const args of [['','la','','','','',true],['Name','','ikeja','','','',true],['Name','','','x'.repeat(256),'','',true],['Name','','','','x'.repeat(2001),'',true],['Name','','','','','http://example.test/photo',true],['Name','','','','','https://user:secret@example.test/photo',true]])assert.throws(()=>profilePayload(...args));
});
test('empty optional fields clear values without changing membership identity',()=>{const p=profilePayload('Member','','','','','',true);assert.equal(p.home_state_id,'');assert.equal(p.avatar_url,'');assert.equal(p.status,undefined);});
test('all membership statuses are explicit and unknown statuses fail closed',()=>{for(const status of ['active','inactive','suspended','revoked','pending_verification','closed'])assert.notEqual(membershipLabel(status),'Not active');assert.equal(membershipLabel('unexpected'),'Not active');});

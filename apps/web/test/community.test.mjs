import test from 'node:test';
import assert from 'node:assert/strict';
import {postPayload,friendActions,memberPath,relationshipPath} from '../src/lib/community.ts';
test('post payload validates Unicode length, audience and whitespace without accepting author fields',()=>{
 assert.deepEqual(postPayload(' Hello ','friends'),{body:'Hello',visibility:'friends'});
 assert.equal(postPayload('🙂'.repeat(3000),'members').body.length,6000);
 for(const args of [[' ','members'],['a'.repeat(3001),'members'],['hello','public']])assert.throws(()=>postPayload(...args));
});
test('friend actions distinguish incoming requests, cancellation and accepted friends',()=>{
 assert.deepEqual(friendActions('incoming').map(a=>a.action),['accept','decline']);
 assert.equal(friendActions('outgoing')[0].label,'Cancel request');
 assert.equal(friendActions('friends')[0].label,'Remove friend');
 assert.equal(friendActions('none')[0].method,'POST');
});
test('member routes encode identifiers and use a separate network namespace',()=>{
 assert.equal(memberPath('ref/a?b'),'/member/people/ref%2Fa%3Fb');
 assert.equal(relationshipPath('ref/a?b'),'/network/members/ref%2Fa%3Fb');
});

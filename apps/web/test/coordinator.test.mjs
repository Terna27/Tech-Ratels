import test from 'node:test';
import assert from 'node:assert/strict';
import {calendarInputToUTC,correctionPayload} from '../src/lib/coordinator.ts';
test('activity scheduling sends a valid UTC timestamp',()=>{assert.equal(calendarInputToUTC('2026-10-28T10:30:00Z'),'2026-10-28T10:30:00.000Z');assert.throws(()=>calendarInputToUTC('invalid'),/valid/);});
test('member correction requires a reason and carries no account authority fields',()=>{assert.throws(()=>correctionPayload('state','lga','Address',' '),/reason/);assert.deepEqual(correctionPayload('state','lga',' Address ',' Member confirmed '),{home_state_id:'state',home_lga_id:'lga',address:'Address',reason:'Member confirmed'});});

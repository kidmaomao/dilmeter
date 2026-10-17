import assert from 'node:assert/strict';
import { openExternalLink } from '../src/lib/openExternalLink.ts';

const host = { __dilmeterOpenExternal: undefined as undefined | ((url: string) => Promise<unknown>) };
Object.assign(globalThis, { window: host });
let prevented = false;
let errorMessage = '';
const event = { currentTarget: { href: 'https://www.noginogi.sbs/article/20260724' }, preventDefault() { prevented = true; } } as unknown as MouseEvent;
await openExternalLink(event, value => { errorMessage = value; });
assert.equal(prevented, false, 'ordinary browser links keep normal navigation');
let destination = '';
host.__dilmeterOpenExternal = async url => { assert.equal(prevented, true); destination = url; };
await openExternalLink(event, value => { errorMessage = value; });
assert.equal(destination, (event.currentTarget as HTMLAnchorElement).href);
assert.equal(errorMessage, '');
host.__dilmeterOpenExternal = async () => { throw new Error('no default handler'); };
await openExternalLink(event, value => { errorMessage = value; });
assert.match(errorMessage, /复制链接/);
console.log('External links verified: native browser dispatch, web fallback and visible failure.');

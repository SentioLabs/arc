import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { importKey, decryptJSON } from './crypto';

const FIXTURES_PATH = join(__dirname, '../../../../internal/paste/testdata/xlang_fixtures.json');
const fixtures: Array<{
	name: string;
	key_b64url: string;
	plaintext: unknown;
	ciphertext_b64: string;
	iv_b64: string;
}> = JSON.parse(readFileSync(FIXTURES_PATH, 'utf8'));

describe('crypto xlang', () => {
	for (const f of fixtures) {
		it(`decrypts Go-produced fixture: ${f.name}`, async () => {
			const key = await importKey(f.key_b64url);
			const blob = stdB64Decode(f.ciphertext_b64);
			const iv = stdB64Decode(f.iv_b64);
			const got = await decryptJSON(blob, iv, key);
			expect(got).toEqual(f.plaintext);
		});
	}
});

function stdB64Decode(s: string): Uint8Array {
	const bin = atob(s);
	const out = new Uint8Array(bin.length);
	for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
	return out;
}

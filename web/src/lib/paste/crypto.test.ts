import { describe, it, expect } from 'vitest';
import {
	generateKey,
	encryptJSON,
	decryptJSON,
	exportKey,
	importKey,
	base64UrlEncode,
	base64UrlDecode
} from './crypto';

describe('paste crypto', () => {
	it('round-trips JSON', async () => {
		const key = await generateKey();
		const value = { hello: 'world', n: 42 };
		const { blob, iv } = await encryptJSON(value, key);
		const decoded = await decryptJSON<typeof value>(blob, iv, key);
		expect(decoded).toEqual(value);
	});

	it('fails to decrypt with wrong key', async () => {
		const k1 = await generateKey();
		const k2 = await generateKey();
		const { blob, iv } = await encryptJSON('secret', k1);
		await expect(decryptJSON(blob, iv, k2)).rejects.toBeDefined();
	});

	it('exports and re-imports key losslessly', async () => {
		const k = await generateKey();
		const exported = await exportKey(k);
		const imported = await importKey(exported);
		const { blob, iv } = await encryptJSON('hi', k);
		const out = await decryptJSON<string>(blob, iv, imported);
		expect(out).toBe('hi');
	});

	it('base64url round-trips arbitrary bytes', () => {
		const bytes = new Uint8Array([0, 255, 1, 254, 100]);
		const decoded = base64UrlDecode(base64UrlEncode(bytes));
		expect([...decoded]).toEqual([...bytes]);
	});
});

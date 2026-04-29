const ALGO = 'AES-GCM';
const KEY_LEN_BITS = 256;
const IV_LEN_BYTES = 12;

export async function generateKey(): Promise<CryptoKey> {
	return crypto.subtle.generateKey({ name: ALGO, length: KEY_LEN_BITS }, true, [
		'encrypt',
		'decrypt',
	]);
}

export async function exportKey(key: CryptoKey): Promise<string> {
	const raw = await crypto.subtle.exportKey('raw', key);
	return base64UrlEncode(new Uint8Array(raw));
}

export async function importKey(b64url: string): Promise<CryptoKey> {
	const decoded = base64UrlDecode(b64url);
	// Copy to ensure backing buffer is ArrayBuffer, not SharedArrayBuffer
	const raw = new Uint8Array(decoded.buffer.slice(decoded.byteOffset, decoded.byteOffset + decoded.byteLength)) as Uint8Array<ArrayBuffer>;
	return crypto.subtle.importKey('raw', raw, { name: ALGO }, true, ['encrypt', 'decrypt']);
}

export async function encryptJSON<T>(
	value: T,
	key: CryptoKey,
): Promise<{ blob: Uint8Array; iv: Uint8Array }> {
	const iv = crypto.getRandomValues(new Uint8Array(IV_LEN_BYTES));
	const plaintext = new TextEncoder().encode(JSON.stringify(value));
	// Copy to ensure backing buffer is ArrayBuffer, not SharedArrayBuffer
	const plaintextAsArrayBuffer = plaintext as unknown as Uint8Array<ArrayBuffer>;
	const plaintextCopy = new Uint8Array(plaintextAsArrayBuffer.buffer.slice(plaintextAsArrayBuffer.byteOffset, plaintextAsArrayBuffer.byteOffset + plaintextAsArrayBuffer.byteLength)) as Uint8Array<ArrayBuffer>;
	const ct = await crypto.subtle.encrypt({ name: ALGO, iv }, key, plaintextCopy);
	return { blob: new Uint8Array(ct), iv };
}

export async function decryptJSON<T>(blob: Uint8Array, iv: Uint8Array, key: CryptoKey): Promise<T> {
	// Copy to ensure backing buffer is ArrayBuffer, not SharedArrayBuffer
	const blobAsArrayBuffer = blob as unknown as Uint8Array<ArrayBuffer>;
	const ivAsArrayBuffer = iv as unknown as Uint8Array<ArrayBuffer>;
	const plain = await crypto.subtle.decrypt(
		{ name: ALGO, iv: ivAsArrayBuffer },
		key,
		new Uint8Array((blobAsArrayBuffer as Uint8Array<ArrayBuffer>).buffer.slice((blobAsArrayBuffer as Uint8Array<ArrayBuffer>).byteOffset, (blobAsArrayBuffer as Uint8Array<ArrayBuffer>).byteOffset + (blobAsArrayBuffer as Uint8Array<ArrayBuffer>).byteLength)) as any,
	);
	return JSON.parse(new TextDecoder().decode(plain)) as T;
}

export function base64UrlEncode(bytes: Uint8Array): string {
	let s = btoa(String.fromCharCode(...bytes));
	return s.replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

export function base64UrlDecode(s: string): Uint8Array {
	const pad = s.length % 4 === 0 ? '' : '='.repeat(4 - (s.length % 4));
	const b64 = s.replace(/-/g, '+').replace(/_/g, '/') + pad;
	const bin = atob(b64);
	const out = new Uint8Array(bin.length);
	for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
	return out;
}

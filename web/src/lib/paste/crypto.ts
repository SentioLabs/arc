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
	const raw = base64UrlDecode(b64url);
	return crypto.subtle.importKey('raw', raw, { name: ALGO }, true, ['encrypt', 'decrypt']);
}

export async function encryptJSON<T>(
	value: T,
	key: CryptoKey,
): Promise<{ blob: Uint8Array; iv: Uint8Array }> {
	const iv = crypto.getRandomValues(new Uint8Array(IV_LEN_BYTES));
	const plaintext = new TextEncoder().encode(JSON.stringify(value));
	const ct = await crypto.subtle.encrypt({ name: ALGO, iv }, key, plaintext);
	return { blob: new Uint8Array(ct), iv };
}

export async function decryptJSON<T>(blob: Uint8Array, iv: Uint8Array, key: CryptoKey): Promise<T> {
	const plain = await crypto.subtle.decrypt({ name: ALGO, iv }, key, blob);
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

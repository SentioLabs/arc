import type {
	CreatePasteResponse,
	GetPasteResponse,
	PasteEventResponse,
} from './types';

export class PasteClient {
	constructor(private baseUrl: string) {}

	async create(planBlob: Uint8Array, planIv: Uint8Array, schemaVer = 1): Promise<CreatePasteResponse> {
		const res = await fetch(`${this.baseUrl}/api/paste`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				plan_blob: bytesToB64(planBlob),
				plan_iv: bytesToB64(planIv),
				schema_ver: schemaVer,
			}),
		});
		if (!res.ok) throw new Error(`create paste failed: ${res.status}`);
		return await res.json();
	}

	async get(id: string): Promise<GetPasteResponse> {
		const res = await fetch(`${this.baseUrl}/api/paste/${id}`);
		if (!res.ok) throw new Error(`get paste failed: ${res.status}`);
		return await res.json();
	}

	async appendEvent(id: string, blob: Uint8Array, iv: Uint8Array): Promise<{ id: string }> {
		const res = await fetch(`${this.baseUrl}/api/paste/${id}/blobs`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ blob: bytesToB64(blob), iv: bytesToB64(iv) }),
		});
		if (!res.ok) throw new Error(`append event failed: ${res.status}`);
		return await res.json();
	}

	async updatePlan(id: string, editToken: string, planBlob: Uint8Array, planIv: Uint8Array): Promise<void> {
		const res = await fetch(`${this.baseUrl}/api/paste/${id}`, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${editToken}` },
			body: JSON.stringify({ plan_blob: bytesToB64(planBlob), plan_iv: bytesToB64(planIv) }),
		});
		if (!res.ok) throw new Error(`update plan failed: ${res.status}`);
	}
}

function bytesToB64(b: Uint8Array): string {
	return btoa(String.fromCharCode(...b));
}

export function b64ToBytes(s: string): Uint8Array {
	const bin = atob(s);
	const out = new Uint8Array(bin.length);
	for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
	return out;
}

export function eventBytes(e: PasteEventResponse): { blob: Uint8Array; iv: Uint8Array } {
	return { blob: b64ToBytes(e.blob), iv: b64ToBytes(e.iv) };
}

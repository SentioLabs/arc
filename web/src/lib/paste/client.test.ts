import { describe, it, expect, vi, beforeEach } from 'vitest';
import { PasteClient } from './client';

describe('PasteClient', () => {
	beforeEach(() => vi.restoreAllMocks());

	it('POSTs base64-encoded blob+iv on create', async () => {
		const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
			new Response(JSON.stringify({ id: 'abc', edit_token: 'tok' }), { status: 201 }),
		);
		const c = new PasteClient('http://x');
		const res = await c.create(new Uint8Array([1, 2]), new Uint8Array([3, 4]));
		expect(res.id).toBe('abc');
		expect(fetchMock).toHaveBeenCalledWith(
			'http://x/api/paste',
			expect.objectContaining({ method: 'POST' }),
		);
		const body = JSON.parse((fetchMock.mock.calls[0][1] as RequestInit).body as string);
		expect(body.schema_ver).toBe(1);
		expect(body.plan_blob).toMatch(/^[A-Za-z0-9+/=]+$/);
	});

	it('sends Bearer token on update', async () => {
		const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
			new Response(null, { status: 204 }),
		);
		const c = new PasteClient('http://x');
		await c.updatePlan('abc', 'mytoken', new Uint8Array([1]), new Uint8Array([2]));
		const opts = fetchMock.mock.calls[0][1] as RequestInit;
		expect((opts.headers as Record<string, string>).Authorization).toBe('Bearer mytoken');
	});
});

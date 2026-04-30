import type { PageLoad } from './$types';

export const ssr = false; // SPA-only; we need window.location.hash and crypto.subtle

export const load: PageLoad = async ({ params }) => {
	return { id: params.id };
};

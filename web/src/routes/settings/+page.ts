import type { PageLoad } from './$types';
import { getConfig } from '$lib/api';

export const load: PageLoad = async () => {
	try {
		const data = await getConfig();
		return { config: data, available: true as const };
	} catch (err) {
		return { config: null, available: false as const, error: String(err) };
	}
};

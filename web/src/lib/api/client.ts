import createClient from 'openapi-fetch';
import type { paths } from './types';

// Create typed API client
// In development, requests are proxied via Vite to the Go server
// In production, the frontend is served from the same origin as the API
const baseUrl = '/api/v1';

export const api = createClient<paths>({ baseUrl });

// Helper type exports for convenience
export type { paths, components } from './types';

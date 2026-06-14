// ============================================================================
// API Client — Axios + TanStack Query setup
// ============================================================================

import axios from 'axios';
import { QueryClient } from '@tanstack/react-query';
import { API } from 'theme/tokens';

// ---------------------------------------------------------------------------
// Axios instance
// ---------------------------------------------------------------------------
const api = axios.create({
  baseURL: API.BASE_URL,
  timeout: API.REQUEST_TIMEOUT,
  headers: { 'Content-Type': 'application/json' },
});

// ---- Request interceptor ------------------------------------------------
api.interceptors.request.use(
  (config) => {
    if (process.env.NODE_ENV === 'development') {
      console.log(`[API] ${config.method?.toUpperCase()} ${config.url}`);
    }
    return config;
  },
  (error) => Promise.reject(error),
);

// ---- Response interceptor -----------------------------------------------
api.interceptors.response.use(
  (response) => response.data,
  (error) => {
    const status = error.response?.status;
    const message =
      error.response?.data?.message ||
      error.message ||
      'Unknown API error';

    if (process.env.NODE_ENV === 'development') {
      console.error(`[API] Error ${status}: ${message}`);
    }

    // Preserve the original error properties for callers that need them
    const enhancedError = new Error(message);
    enhancedError.status = status;
    enhancedError.response = error.response;
    enhancedError.originalError = error;

    return Promise.reject(enhancedError);
  },
);

// ---------------------------------------------------------------------------
// TanStack Query client
// ---------------------------------------------------------------------------
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30 * 1000,    // 30s
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
});

export default api;
export { api, queryClient };

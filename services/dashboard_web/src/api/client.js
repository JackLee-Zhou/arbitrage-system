// ============================================================================
// API Client — Axios + TanStack Query + Endpoint wrappers
// ============================================================================

import axios from 'axios';
import { QueryClient } from '@tanstack/react-query';
import { API } from 'theme/tokens';

// ---- Axios instance --------------------------------------------------------
const api = axios.create({
  baseURL: API.BASE_URL,
  timeout: API.REQUEST_TIMEOUT,
  headers: { 'Content-Type': 'application/json' },
});

api.interceptors.request.use(
  (config) => {
    if (process.env.NODE_ENV === 'development') {
      console.log(`[API] ${config.method?.toUpperCase()} ${config.url}`);
    }
    return config;
  },
  (error) => Promise.reject(error),
);

api.interceptors.response.use(
  (response) => response.data,
  (error) => {
    const status = error.response?.status;
    const message = error.response?.data?.message || error.message || 'Unknown API error';

    if (process.env.NODE_ENV === 'development') {
      console.error(`[API] Error ${status}: ${message}`);
    }

    const enhancedError = new Error(message);
    enhancedError.status = status;
    enhancedError.response = error.response;
    enhancedError.originalError = error;

    return Promise.reject(enhancedError);
  },
);

// ---- Query Client ----------------------------------------------------------
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30 * 1000,
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
});

// ---- API Endpoints ---------------------------------------------------------

/** GET /api/v1/overview — 概览页聚合数据 */
export function fetchOverview() {
  return api.get('/api/v1/overview').catch(() => {
    // Fallback: return empty structure when API is unavailable
    return {
      status: 'offline',
      system_mode: 'unknown',
      services: {},
      positions: { positions: [] },
      ai_score: null,
      today_stats: { orders: 0, trades: 0, pnl: 0 },
    };
  });
}

/** GET /api/v1/positions — 持仓数据，可选 ?symbol=MA2509 */
export function fetchPositions(symbol) {
  const params = symbol ? { symbol } : {};
  return api.get('/api/v1/positions', { params }).catch(() => ({
    positions: [],
  }));
}

/** GET /api/v1/services — 服务注册列表 */
export function fetchServices() {
  return api.get('/api/v1/services').catch(() => ({}));
}

/** POST /api/v1/emergency_close — 紧急平仓 */
export function emergencyClose(symbol, reason, scope) {
  return api.post('/api/v1/emergency_close', { symbol, reason, scope }).catch((err) => {
    throw err;
  });
}

/** GET /api/v1/contracts — 合约列表，可选 ?type=main */
export function fetchContracts(type) {
  const params = type ? { type } : {};
  return api.get('/api/v1/contracts', { params }).catch(() => []);
}

/** GET /api/v1/contracts/{symbol}/snapshot — 合约快照 */
export function fetchContractSnapshot(symbol) {
  return api.get(`/api/v1/contracts/${symbol}/snapshot`).catch(() => null);
}

/** GET /api/v1/ai/scores/current — AI 评分，可选 ?symbol=xxx */
export function fetchAIScores(symbol) {
  const params = symbol ? { symbol } : {};
  return api.get('/api/v1/ai/scores/current', { params }).catch(() => null);
}

/** GET /api/v1/strategies — 策略列表 */
export function fetchStrategies() {
  return api.get('/api/v1/strategies').then(data => {
    // Normalize: handler returns StatusResponse or {strategies: [...]}
    return { strategies: data.strategies || [] };
  }).catch(() => ({ strategies: [] }));
}

/** GET /api/v1/risk — 风控参数 */
export function fetchRiskParams() {
  return api.get('/api/v1/risk').then(data => {
    // Normalize: handler returns StatusResponse or {risk_params: {...}}
    return { risk_params: data.risk_params || {} };
  }).catch(() => ({ risk_params: {} }));
}

export default api;
export { api, queryClient };

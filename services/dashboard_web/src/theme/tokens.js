// ============================================================================
// Design Tokens — Precision Dark Trading Terminal
// ============================================================================

// ---- Color System ----------------------------------------------------------
export const COLORS = {
  // Primary accents
  CYAN: '#00E5FF',           // Primary action / data highlight
  CYAN_DIM: '#00B8D4',       // Hover / secondary
  EMERALD: '#00E676',         // Bullish / profit / healthy — green shift
  CRIMSON: '#FF1744',         // Bearish / loss / alert — pure red
  AMBER: '#FFAB00',           // Warning / stale data
  VIOLET: '#B388FF',          // AI / sentiment — softer than pure purple
  GOLD: '#FFD740',            // Emphasis / pinned / active

  // Chart
  CHART_LINE: '#00E5FF',
  CHART_AREA_TOP: 'rgba(0,229,255,0.18)',
  CHART_AREA_BOTTOM: 'rgba(0,229,255,0)',

  // Semantic
  TRADING_UP: '#00E676',
  TRADING_DOWN: '#FF1744',
  AI_SCORE: '#B388FF',
  AI_STALE: '#FFAB00',
  EMERGENCY: '#FF1744',

  // Background layers
  BG_ROOT: '#070B14',
  BG_SURFACE: '#0D1321',
  BG_ELEVATED: '#141E30',
  BG_OVERLAY: 'rgba(13,19,33,0.85)',

  // Border
  BORDER_SUBTLE: 'rgba(255,255,255,0.06)',
  BORDER_ACTIVE: 'rgba(0,229,255,0.25)',
};

// ---- Glow Effects -----------------------------------------------------------
export const GLOW = {
  CARD: '0 0 20px rgba(0,229,255,0.06)',
  CARD_HOVER: '0 0 30px rgba(0,229,255,0.12)',
  BUTTON: '0 0 12px rgba(0,229,255,0.25)',
  EMERGENCY_BTN: '0 0 24px rgba(255,23,68,0.45)',
  DATA_LIVE: '0 0 8px rgba(0,230,118,0.30)',
};

// ---- Timing (ms) ------------------------------------------------------------
export const TIMING = {
  PRICE_FLASH: 300,
  CARD_HOVER: 200,
  WS_RECONNECT_BASE: 1000,
  WS_RECONNECT_MAX: 30000,
};

// ---- Layout Constants (px) ---------------------------------------------------
export const SIZING = {
  SIDEBAR_WIDTH: 250,
  SIDEBAR_COLLAPSED: 80,
  TOPBAR_HEIGHT: 64,
};

// ---- API --------------------------------------------------------------------
export const API = {
  BASE_URL: process.env.REACT_APP_API_URL || 'http://localhost:8080',
  WS_URL: process.env.REACT_APP_WS_URL || 'ws://localhost:8080/ws',
  REQUEST_TIMEOUT: 10000,
};

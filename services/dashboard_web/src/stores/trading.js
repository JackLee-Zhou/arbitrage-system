import { create } from 'zustand';

const MAX_ORDERS = 500;
const MAX_TRADES = 500;

const useTradingStore = create((set) => ({
  positions: {},
  totalFloatPnL: 0,
  aiScores: {},
  todayOrders: [],
  todayTrades: [],
  selectedSymbol: null,

  setPositions: (positions) =>
    set(() => {
      let totalFloatPnL = 0;
      if (typeof positions === 'object' && positions !== null) {
        for (const key of Object.keys(positions)) {
          const pos = positions[key];
          if (pos && typeof pos.floatPnL === 'number') {
            totalFloatPnL += pos.floatPnL;
          }
        }
      }
      return { positions, totalFloatPnL };
    }),

  updatePosition: (symbol, data) => {
    if (!symbol || !data) return;
    set((state) => {
      const updated = {
        ...state.positions,
        [symbol]: {
          ...(state.positions[symbol] || {}),
          ...data,
          symbol,
        },
      };
      let totalFloatPnL = 0;
      for (const key of Object.keys(updated)) {
        const pos = updated[key];
        if (pos && typeof pos.floatPnL === 'number') {
          totalFloatPnL += pos.floatPnL;
        }
      }
      return { positions: updated, totalFloatPnL };
    });
  },

  removePosition: (symbol) =>
    set((state) => {
      const { [symbol]: _removed, ...rest } = state.positions;
      let totalFloatPnL = 0;
      for (const key of Object.keys(rest)) {
        const pos = rest[key];
        if (pos && typeof pos.floatPnL === 'number') {
          totalFloatPnL += pos.floatPnL;
        }
      }
      return { positions: rest, totalFloatPnL };
    }),

  setAIScore: (symbol, score) => {
    if (!symbol || !score) return;
    set((state) => ({
      aiScores: {
        ...state.aiScores,
        [symbol]: {
          modelChain: [],
          keyDrivers: [],
          ...(state.aiScores[symbol] || {}),
          ...score,
        },
      },
    }));
  },

  setAIScores: (scores) =>
    set((state) => {
      const merged = { ...state.aiScores };
      if (typeof scores === 'object' && scores !== null) {
        for (const key of Object.keys(scores)) {
          merged[key] = {
            modelChain: [],
            keyDrivers: [],
            ...(merged[key] || {}),
            ...scores[key],
          };
        }
      }
      return { aiScores: merged };
    }),

  addOrder: (order) => {
    if (!order) return;
    set((state) => ({
      todayOrders: [
        { ...order, timestamp: order.timestamp ?? new Date().toISOString() },
        ...state.todayOrders,
      ].slice(0, MAX_ORDERS),
    }));
  },

  addTrade: (trade) => {
    if (!trade) return;
    set((state) => ({
      todayTrades: [
        { ...trade, timestamp: trade.timestamp ?? new Date().toISOString() },
        ...state.todayTrades,
      ].slice(0, MAX_TRADES),
    }));
  },

  selectSymbol: (symbol) =>
    set({ selectedSymbol: symbol }),
}));

export default useTradingStore;

import { create } from 'zustand';

const MAX_ALERTS = 100;

const useSystemStore = create((set) => ({
  services: {},
  systemMode: 'normal',
  alerts: [],
  wsStatus: 'disconnected',

  setServices: (services) =>
    set({ services }),

  updateService: (name, data) =>
    set((state) => ({
      services: {
        ...state.services,
        [name]: {
          ...state.services[name],
          ...data,
        },
      },
    })),

  setSystemMode: (mode) =>
    set({ systemMode: mode }),

  addAlert: (alert) =>
    set((state) => {
      if (!alert) return state;
      const next = [
        { ...alert, timestamp: alert.timestamp ?? new Date().toISOString() },
        ...state.alerts,
      ];
      return { alerts: next.slice(0, MAX_ALERTS) };
    }),

  clearAlerts: () =>
    set({ alerts: [] }),

  setWSStatus: (status) =>
    set({ wsStatus: status }),
}));

export default useSystemStore;

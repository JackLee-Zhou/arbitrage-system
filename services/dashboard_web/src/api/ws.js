// ============================================================================
// WebSocket connection manager — auto-reconnect + subscribe protocol
// ============================================================================

import { API, TIMING } from 'theme/tokens';

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------
const DEFAULT_CHANNELS = ['system_status', 'positions', 'scores', 'alerts'];
const PING_INTERVAL_MS = 60_000;

// ---------------------------------------------------------------------------
// createWS — WebSocket manager factory
// ---------------------------------------------------------------------------

/**
 * @param {string} url — WebSocket endpoint
 * @returns {{
 *   open: (channels?: string[]) => void,
 *   subscribe: (channels: string[]) => void,
 *   unsubscribe: (channels: string[]) => void,
 *   close: () => void,
 *   onMessage: (handler: (msg: object) => void) => () => void,
 *   onStatusChange: (handler: (status: string) => void) => () => void,
 * }}
 */
export function createWS(url) {
  // ---- internal state ---------------------------------------------------
  let ws = null;
  let reconnectAttempt = 0;
  let reconnectTimer = null;
  let pingTimer = null;
  let lastMessageTs = Date.now();
  let intendedClose = false;

  /** @type {Set<(msg: object) => void>} */
  const messageHandlers = new Set();
  /** @type {Set<(status: string) => void>} */
  const statusHandlers = new Set();

  /** @type {string[]} */
  let subscribedChannels = [];

  // ---- helpers ----------------------------------------------------------

  function notifyStatus(status) {
    statusHandlers.forEach((fn) => fn(status));
  }

  function notifyMessage(msg) {
    messageHandlers.forEach((fn) => fn(msg));
  }

  function getBackoffDelay() {
    const delay = TIMING.WS_RECONNECT_BASE * Math.pow(2, reconnectAttempt);
    return Math.min(delay, TIMING.WS_RECONNECT_MAX);
  }

  function clearTimers() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
    if (pingTimer) {
      clearInterval(pingTimer);
      pingTimer = null;
    }
  }

  function closeExistingWS() {
    if (!ws) return;
    // Prevent the old socket's onclose from triggering reconnect
    ws.onmessage = null;
    ws.onerror = null;
    ws.onclose = null;
    try {
      ws.close();
    } catch (_) {
      // Already closing — ignore
    }
    ws = null;
  }

  // ---- ping/pong watchdog ------------------------------------------------

  function startPingWatchdog() {
    stopPingWatchdog();
    lastMessageTs = Date.now();

    pingTimer = setInterval(() => {
      if (ws && ws.readyState === WebSocket.OPEN) {
        const elapsed = Date.now() - lastMessageTs;
        if (elapsed >= PING_INTERVAL_MS) {
          try {
            ws.send(JSON.stringify({ type: 'ping' }));
          } catch (_) {
            // ignore send failures — the next error/close will trigger reconnect
          }
        }
      }
    }, 10_000); // check every 10s
  }

  function stopPingWatchdog() {
    if (pingTimer) {
      clearInterval(pingTimer);
      pingTimer = null;
    }
  }

  // ---- subscribe protocol ------------------------------------------------

  function sendSubscribe(channels) {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(
        JSON.stringify({
          type: 'subscribe',
          channels,
        }),
      );
    }
  }

  function sendUnsubscribe(channels) {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(
        JSON.stringify({
          type: 'unsubscribe',
          channels,
        }),
      );
    }
  }

  // ---- connection lifecycle ----------------------------------------------

  function connect() {
    if (intendedClose) return;

    // Clean up any previous connection before opening a new one
    closeExistingWS();

    notifyStatus('connecting');

    try {
      ws = new WebSocket(url);
    } catch (err) {
      notifyStatus('error');
      scheduleReconnect();
      return;
    }

    ws.onopen = () => {
      reconnectAttempt = 0;
      notifyStatus('connected');
      startPingWatchdog();

      // Re-subscribe to current channels
      if (subscribedChannels.length > 0) {
        sendSubscribe(subscribedChannels);
      }
    };

    ws.onmessage = (event) => {
      lastMessageTs = Date.now();

      try {
        const msg = JSON.parse(event.data);

        // Handle server pong (no-op, just resets the watchdog timer)
        if (msg.type === 'pong') return;

        notifyMessage(msg);
      } catch (_) {
        // non-JSON message — ignore
      }
    };

    ws.onerror = () => {
      notifyStatus('error');
    };

    ws.onclose = () => {
      stopPingWatchdog();
      notifyStatus('disconnected');

      if (!intendedClose) {
        scheduleReconnect();
      }
    };
  }

  function scheduleReconnect() {
    clearTimers();
    const delay = getBackoffDelay();
    reconnectAttempt += 1;
    reconnectTimer = setTimeout(() => connect(), delay);
  }

  // ---- public API --------------------------------------------------------

  /** Open (or re-open) the connection with a set of channels */
  function open(channels = DEFAULT_CHANNELS) {
    intendedClose = false;
    subscribedChannels = [...channels];
    reconnectAttempt = 0;
    connect();
  }

  /** Subscribe to additional channels */
  function subscribe(channels) {
    const newChannels = channels.filter((ch) => !subscribedChannels.includes(ch));
    if (newChannels.length === 0) return;

    subscribedChannels = [...subscribedChannels, ...newChannels];
    sendSubscribe(newChannels);
  }

  /** Unsubscribe from channels */
  function unsubscribe(channels) {
    subscribedChannels = subscribedChannels.filter((ch) => !channels.includes(ch));
    sendUnsubscribe(channels);
  }

  /** Gracefully close the connection */
  function close() {
    intendedClose = true;
    clearTimers();
    subscribedChannels = [];
    closeExistingWS();
    notifyStatus('disconnected');
  }

  /** Register a message handler. Returns an unsubscribe function. */
  function onMessage(handler) {
    messageHandlers.add(handler);
    return () => messageHandlers.delete(handler);
  }

  /** Register a status-change handler. Returns an unsubscribe function. */
  function onStatusChange(handler) {
    statusHandlers.add(handler);
    return () => statusHandlers.delete(handler);
  }

  return { open, subscribe, unsubscribe, close, onMessage, onStatusChange };
}

// ---------------------------------------------------------------------------
// Singleton — lazily connected to avoid module-level side effects
// ---------------------------------------------------------------------------
let _wsManager = null;

function getWSManager() {
  if (!_wsManager) {
    _wsManager = createWS(API.WS_URL);
  }
  return _wsManager;
}

export default getWSManager;

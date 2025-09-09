export type StreamMessage = any;

let source: EventSource | null = null;
let listeners: Array<(msg: StreamMessage) => void> = [];
let reconnectTimer: number | null = null;
let getToken: (() => Promise<string>) | null = null;
let url = "";
let connecting = false;

async function connect() {
  if (!getToken) return;
  try {
    const token = await getToken();
    const encoded = encodeURIComponent(token);
    source = new EventSource(`${url}?token=${encoded}`);
    source.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data);
        listeners.forEach((l) => l(msg));
      } catch (e) {
        console.error(e);
      }
    };
    source.onerror = () => {
      source?.close();
      reconnectTimer = window.setTimeout(connect, 5000);
    };
  } catch (err) {
    console.error(err);
  } finally {
    connecting = false;
  }
}

export function subscribe(
  tokenProvider: () => Promise<string>,
  streamUrl: string,
  handler: (msg: StreamMessage) => void
): () => void {
  getToken = tokenProvider;
  url = streamUrl;
  listeners.push(handler);
  if (!source && !connecting) {
    connecting = true;
    void connect();
  }
  return () => {
    listeners = listeners.filter((l) => l !== handler);
    if (listeners.length === 0) {
      if (reconnectTimer) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
      source?.close();
      source = null;
    }
  };
}

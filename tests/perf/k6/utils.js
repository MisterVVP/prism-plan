import http from 'k6/http';
import { SharedArray } from 'k6/data';
import { gzip } from './vendor/pako-gzip.mjs';

const tokens = new SharedArray('tokens', () => {
  try {
    const contents = open('./bearers.json');
    if (!contents) {
      return [];
    }
    const parsed = JSON.parse(contents);
    if (!Array.isArray(parsed)) {
      return [];
    }
    return parsed
      .filter((token) => typeof token === 'string')
      .map((token) => token.trim())
      .filter((token) => token.length > 0);
  } catch (err) {
    return [];
  }
});

function selectBearerFromTokens() {
  if (!tokens.length) {
    return '';
  }
  const index = (__VU - 1) % tokens.length;
  const token = tokens[index];
  return typeof token === 'string' ? token.trim() : '';
}

function resolveBearer() {
  const bearerFromTokens = selectBearerFromTokens();
  if (bearerFromTokens) {
    return bearerFromTokens;
  }
  if (typeof __ENV.K6_BEARER === 'string' && __ENV.K6_BEARER.trim()) {
    return __ENV.K6_BEARER.trim();
  }
  return '';
}

export function buildAuthHeaders() {
  const bearer = resolveBearer();
  const headers = {
    Accept: 'application/json',
    'Accept-Encoding': 'gzip',
  };
  if (bearer) {
    headers.Authorization = `Bearer ${bearer}`;
  }
  return headers;
}

export function fetchAllTasks(base, headers) {
  let pageToken = '';
  const seenTokens = new Set();
  while (true) {
    const query = pageToken ? `?pageToken=${encodeURIComponent(pageToken)}` : '';
    const res = http.get(`${base}/api/tasks${query}`, {
      headers,
      tags: { endpoint: '/api/tasks' },
    });
    if (res.status !== 200) {
      return;
    }
    let payload;
    try {
      payload = JSON.parse(res.body);
    } catch (err) {
      return;
    }
    const nextToken =
      typeof payload.nextPageToken === 'string'
        ? payload.nextPageToken.trim()
        : '';
    if (!nextToken) {
      return;
    }
    if (seenTokens.has(nextToken)) {
      return;
    }
    seenTokens.add(nextToken);
    pageToken = nextToken;
  }
}

function toArrayBuffer(view) {
  if (view.byteOffset === 0 && view.byteLength === view.buffer.byteLength) {
    return view.buffer;
  }
  return view.buffer.slice(view.byteOffset, view.byteOffset + view.byteLength);
}

export function gzipJSONPayload(payload) {
  const json = typeof payload === 'string' ? payload : JSON.stringify(payload);
  const compressed = gzip(json, { level: 1 });
  return toArrayBuffer(compressed);
}


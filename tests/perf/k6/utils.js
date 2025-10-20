import http from 'k6/http';
import { SharedArray } from 'k6/data';

const MAX_PAGE_SIZE = 1000;
const DEFAULT_PAGE_SIZE = 250;

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
  if (!bearer) {
    return {};
  }
  return { Authorization: `Bearer ${bearer}` };
}

export function fetchAllTasks(base, headers) {
  const resolvedPageSize = resolvePageSize();
  let pageToken = '';
  const seenTokens = new Set();
  while (true) {
    const params = [];
    if (resolvedPageSize > 0) {
      params.push(`pageSize=${resolvedPageSize}`);
    }
    if (pageToken) {
      params.push(`pageToken=${encodeURIComponent(pageToken)}`);
    }
    const query = params.length ? `?${params.join('&')}` : '';
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

function resolvePageSize() {
  const raw = Number(__ENV.K6_TASK_PAGE_SIZE);
  if (Number.isFinite(raw) && raw > 0) {
    const floored = Math.floor(raw);
    if (floored > 0) {
      return Math.min(floored, MAX_PAGE_SIZE);
    }
  }
  return DEFAULT_PAGE_SIZE;
}

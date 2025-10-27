import http from 'k6/http';
import { SharedArray } from 'k6/data';

const MAX_PAGE_SIZE = 1000;
const DEFAULT_PAGE_SIZE = 250;
const DEFAULT_ARRIVAL_RATE = 10;
const DEFAULT_PRE_ALLOCATED_VUS = 200;
const DEFAULT_MAX_VUS = 1000;
const MAX_ALLOWED_VUS = 10000;
const DEFAULT_TIME_UNIT = '1s';
const DEFAULT_DURATION = '30s';
const warnedLegacyEnv = new Set();

function pickEnvValue(keys) {
  for (const key of keys) {
    if (Object.prototype.hasOwnProperty.call(__ENV, key)) {
      return { key, value: __ENV[key] };
    }
  }
  return { key: undefined, value: undefined };
}

function warnIfLegacyEnv(key, canonical) {
  if (!key || key === canonical) {
    return;
  }
  if (warnedLegacyEnv.has(key)) {
    return;
  }
  console.warn(
    `Environment variable "${key}" is deprecated; use "${canonical}" instead for Prism perf tests.`,
  );
  warnedLegacyEnv.add(key);
}

function resolvePositiveIntegerEnv(keys, fallback) {
  const [canonical] = keys;
  const { key, value } = pickEnvValue(keys);
  warnIfLegacyEnv(key, canonical);
  return parsePositiveInteger(value, fallback);
}

function resolveStringEnvFromKeys(keys, fallback) {
  const [canonical] = keys;
  const { key, value } = pickEnvValue(keys);
  warnIfLegacyEnv(key, canonical);
  return resolveStringEnv(value, fallback);
}

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

function parsePositiveInteger(value, fallback) {
  const parsed = Number(value);
  if (Number.isFinite(parsed) && parsed > 0) {
    return Math.floor(parsed);
  }
  return fallback;
}

function resolveStringEnv(value, fallback) {
  if (typeof value === 'string') {
    const trimmed = value.trim();
    if (trimmed) {
      return trimmed;
    }
  }
  return fallback;
}

export function buildOpenModelScenario(overrides = {}) {
  const rate = resolvePositiveIntegerEnv(
    ['PRISM_K6_ARRIVAL_RATE', 'K6_ARRIVAL_RATE'],
    DEFAULT_ARRIVAL_RATE,
  );
  const preAllocatedVUs = resolvePositiveIntegerEnv(
    ['PRISM_K6_PRE_ALLOCATED_VUS', 'K6_PRE_ALLOCATED_VUS'],
    DEFAULT_PRE_ALLOCATED_VUS,
  );
  let maxVUs = resolvePositiveIntegerEnv(
    ['PRISM_K6_MAX_VUS', 'K6_MAX_VUS'],
    Math.max(preAllocatedVUs, DEFAULT_MAX_VUS),
  );
  if (maxVUs > MAX_ALLOWED_VUS) {
    maxVUs = MAX_ALLOWED_VUS;
  }
  if (maxVUs < preAllocatedVUs) {
    maxVUs = preAllocatedVUs;
  }
  const timeUnit = resolveStringEnvFromKeys(
    ['PRISM_K6_TIME_UNIT', 'K6_TIME_UNIT'],
    DEFAULT_TIME_UNIT,
  );
  const duration = resolveStringEnvFromKeys(
    ['PRISM_K6_DURATION', 'K6_DURATION'],
    DEFAULT_DURATION,
  );

  return {
    executor: 'constant-arrival-rate',
    rate,
    timeUnit,
    duration,
    preAllocatedVUs,
    maxVUs,
    ...overrides,
  };
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
  const { key, value } = pickEnvValue(['PRISM_K6_TASK_PAGE_SIZE', 'K6_TASK_PAGE_SIZE']);
  warnIfLegacyEnv(key, 'PRISM_K6_TASK_PAGE_SIZE');
  const raw = Number(value);
  if (Number.isFinite(raw) && raw > 0) {
    const floored = Math.floor(raw);
    if (floored > 0) {
      return Math.min(floored, MAX_PAGE_SIZE);
    }
  }
  return DEFAULT_PAGE_SIZE;
}

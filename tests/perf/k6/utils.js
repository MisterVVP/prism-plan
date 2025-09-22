import { SharedArray } from 'k6/data';

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


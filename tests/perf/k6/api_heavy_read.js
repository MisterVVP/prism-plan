import http from 'k6/http';
import { SharedArray } from 'k6/data';

const tokens = new SharedArray('tokens', () => JSON.parse(open('./bearers.json')));

export const options = {
  scenarios: {
    default: {
      executor: 'constant-vus',
      vus: Number(__ENV.K6_VUS) || 10,
      duration: __ENV.K6_DURATION || '30s',
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<300', 'p(99)<500'],
  },
};

export default function () {
  const base = __ENV.PRISM_API_LB_BASE || 'http://localhost';
  const bearer = tokens[__VU - 1];
  const headers = {};
  const functionsKey = __ENV.PRISM_API_FUNCTION_KEY || '';
  if (functionsKey) {
    headers['x-functions-key'] = functionsKey;
  }
  if (bearer) {
    headers.Authorization = `Bearer ${bearer}`;
  }
  http.get(`${base}/api/tasks`, { headers, tags: { endpoint: '/api/tasks' }  });
}


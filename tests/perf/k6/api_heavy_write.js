import http from 'k6/http';
import { buildAuthHeaders } from './utils.js';

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
  const headers = buildAuthHeaders();
  const cmd = [
    {
      idempotencyKey: `k6-${__VU}-${Date.now()}-${Math.random()}`,
      entityType: 'task',
      type: 'create-task',
      data: { title: 'k6 task' },
    },
  ];
  const body = JSON.stringify(cmd);
  http.post(`${base}/api/commands`, body, {
    headers: { ...headers, 'Content-Type': 'application/json' },
    compression: 'gzip',
    tags: { endpoint: '/api/commands' },
  });
}


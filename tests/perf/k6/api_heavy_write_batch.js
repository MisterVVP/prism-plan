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
  const postHeaders = Object.assign({ 'Content-Type': 'application/json' }, headers);
  const batchSize = Number(__ENV.K6_COMMANDS_PER_REQUEST) || 8;
  const now = Date.now();
  const cmds = Array.from({ length: batchSize }, (_, idx) => ({
    idempotencyKey: `k6-batch-${__VU}-${now}-${idx}-${Math.random()}`,
    entityType: 'task',
    type: 'create-task',
    data: { title: `k6 task ${idx}` },
  }));
  http.post(
    `${base}/api/commands`,
    JSON.stringify(cmds),
    { headers: postHeaders, tags: { endpoint: '/api/commands' } },
  );
}

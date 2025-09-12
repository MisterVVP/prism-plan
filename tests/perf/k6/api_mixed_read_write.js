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
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
  },
};

export default function () {
  const base = __ENV.PRISM_API_LB_BASE || 'http://localhost';
  const bearer = tokens[__VU - 1];
  const headers = bearer ? { Authorization: `Bearer ${bearer}` } : {};
  if (Math.random() < 0.8) {
    http.get(`${base}/api/tasks`, { headers });
  } else {
    const postHeaders = Object.assign({ 'Content-Type': 'application/json' }, headers);
    const cmd = [
      {
        idempotencyKey: `k6-${__VU}-${Date.now()}-${Math.random()}`,
        entityType: 'task',
        type: 'create-task',
        data: { title: 'k6 task' },
      },
    ];
    http.post(`${base}/api/commands`, JSON.stringify(cmd), { headers: postHeaders });
  }
}


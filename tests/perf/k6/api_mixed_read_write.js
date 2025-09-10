import http from 'k6/http';
import { check } from 'k6';

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
    http_req_duration: ['p(95)<300', 'p(99)<800'],
  },
};

export default function () {
  const base = __ENV.PRISM_API_BASE || 'http://localhost';
  const bearer = __ENV.TEST_BEARER;
  const headers = {};
  if (bearer) {
    headers.Authorization = `Bearer ${bearer}`;
  }
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


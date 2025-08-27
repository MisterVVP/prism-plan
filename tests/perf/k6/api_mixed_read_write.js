import http from 'k6/http';
import { check } from 'k6';

export const options = {
  scenarios: {
    default: {
      executor: 'constant-vus',
      vus: 1,
      duration: '1s',
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<300', 'p(99)<800'],
  },
};

export default function () {
  const base = __ENV.API_BASE || 'http://localhost';
  if (Math.random() < 0.8) {
    http.get(`${base}/api/tasks`);
  } else {
    http.post(`${base}/api/commands`, JSON.stringify([]), { headers: { 'Content-Type': 'application/json' } });
  }
}


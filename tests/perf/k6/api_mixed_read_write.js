import http from 'k6/http';
import { check } from 'k6';

export const options = {
  scenarios: {
    default: {
      executor: 'constant-vus',
      vus: 10,
      duration: '30s',
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
    http.post(
      `${base}/api/commands`,
      JSON.stringify({ type: 'CreateTask', payload: { title: 'k6 task' } }),
      { headers: postHeaders },
    );
  }
}


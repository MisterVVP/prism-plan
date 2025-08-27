import http from 'k6/http';

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
    http_req_duration: ['p(95)<200', 'p(99)<600'],
  },
};

export default function () {
  const base = __ENV.API_BASE || 'http://localhost';
  http.get(`${base}/api/tasks`);
}


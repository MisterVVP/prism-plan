import http from 'k6/http';

export const options = {
  scenarios: {
    default: {
      executor: 'constant-vus',
      vus: 5,
      duration: '1h',
    },
  },
};

export default function () {
  const base = __ENV.API_BASE || 'http://localhost';
  http.get(`${base}/api/tasks`);
}


import http from 'k6/http';

export const options = {
  scenarios: {
    default: {
      executor: 'constant-vus',
      vus: 1,
      duration: '1s',
    },
  },
};

export default function () {
  const base = __ENV.API_BASE || 'http://localhost';
  http.get(`${base}/api/tasks`);
}


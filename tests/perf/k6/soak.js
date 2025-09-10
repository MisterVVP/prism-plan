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
  const base = __ENV.PRISM_API_BASE || 'http://localhost';
  const bearer = __ENV.TEST_BEARER;
  const headers = {};
  if (bearer) {
    headers.Authorization = `Bearer ${bearer}`;
  }
  http.get(`${base}/api/tasks`, { headers });
}


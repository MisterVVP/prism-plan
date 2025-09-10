import http from 'k6/http';

export const options = {
  scenarios: {
    default: {
      executor: 'constant-vus',
      vus: Number(__ENV.K6_VUS) || 5,
      duration: __ENV.K6_DURATION || '1h',
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


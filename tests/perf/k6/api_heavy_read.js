import { buildAuthHeaders, buildOpenModelScenario, fetchAllTasks } from './utils.js';

export const options = {
  scenarios: {
    default: buildOpenModelScenario(),
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<300', 'p(99)<500'],
  },
};

export default function () {
  const base = __ENV.PRISM_API_LB_BASE || 'http://localhost';
  const headers = buildAuthHeaders();
  fetchAllTasks(base, headers);
}


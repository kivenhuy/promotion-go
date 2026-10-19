import http from 'k6/http';

export const options = {
  scenarios: { steady: {
    executor: 'constant-arrival-rate',
    rate: Number(__ENV.RATE || 400), timeUnit: '1s', duration: '60s',
    preAllocatedVUs: 100, maxVUs: 500,
  } },
};

const BASE = __ENV.BASE || 'http://localhost:8090';
http.setResponseCallback(http.expectedStatuses(201, 409));

export default function () {
  const userID = Math.floor(Math.random() * 1000000) + 1;
  http.post(`${BASE}/draws`,
    JSON.stringify({ user_id: userID, campaign: 'tet2026' }),
    { headers: { 'Content-Type': 'application/json', 'Idempotency-Key': `s-${__VU}-${__ITER}-${Date.now()}` } });
}

import http from 'k6/http';
import { check } from 'k6';

// 3 mốc request cố định , mỗi mốc 30s chạy liên tục
// 
const level = (rate, startTime) => ({
    executor: 'constant-arrival-rate',
    rate, timeUnit: '1s', duration: '30s', startTime,
    preAllocatedVUs: 200, maxVUs: 1000,
});

export const options = {
    scenarios: {
        r400: level(400, '0s'),
        r600: level(600, '30s'),
        r800: level(800, '60s'),
    },
    thresholds: {
        'http_req_duration{scenario:r400}': ['p(99)<500'],
        'http_req_duration{scenario:r600}': ['p(99)<500'],
        'http_req_duration{scenario:r800}': ['p(99)<500'],
        http_req_failed: ['rate<0.01'],
        dropped_iterations: ['count<1'],
    },
};

const BASE = __ENV.BASE || 'http://localhost:8090';
http.setResponseCallback(http.expectedStatuses(201, 409));

export default function () {
    const userID = Math.floor(Math.random() * 1000000) + 1;
    const key = `k6-${__VU}-${__ITER}-${Date.now()}`;
    const res = http.post(`${BASE}/draws`,
        JSON.stringify({ user_id: userID, campaign: 'tet2026' }),
        { headers: { 'Content-Type': 'application/json', 'Idempotency-Key': key } });
    check(res, { 'status 201 or 409': r => r.status === 201 || r.status === 409 });
}

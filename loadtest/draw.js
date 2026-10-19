import http from 'k6/http';
import { check } from 'k6';

export const options = {
    stages: [
        { duration: '20s', target: 20 },
        { duration: '20s', target: 50 },
        { duration: '20s', target: 100 },
        { duration: '10s', target: 0 },
    ],
    thresholds: {
        http_req_duration: ['p(99)<500'],
        http_req_failed: ['rate<0.01'],
    },
};

const BASE = __ENV.BASE || 'http://localhost:8090';

http.setResponseCallback(http.expectedStatuses(201, 409));

export default function () {
    const userID = Math.floor(Math.random() * 100000) + 1;
    const key = `k6-${__VU}-${__ITER}-${Date.now()}`;
    const res = http.post(`${BASE}/draws`,
        JSON.stringify({ user_id: userID, campaign: 'tet2026' }),
        { headers: { 'Content-Type': 'application/json', 'Idempotency-Key': key } });
    check(res, { 'status 201 or 409': r => r.status === 201 || r.status === 409 });
}

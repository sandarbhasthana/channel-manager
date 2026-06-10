const http = require('http');

const req = http.request({
  hostname: 'localhost',
  port: 8080,
  path: '/pms.v1.PmsService/ListBookings',
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Authorization': 'Bearer test-token' // this won't work, we need a real cookie
  }
});

# Test WebSocket Connect Method
# This file tests the connection establishment to a WebSocket endpoint

# First, we check that the WebSocket API endpoint is available via HTTP
GET {{ws_base_url}}
HTTP/1.1 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.status" == "ok"

# WebSocket connection simulation
# Since Hurl doesn't directly support WebSocket connections,
# this request simulates checking WebSocket availability
GET {{ws_status_endpoint}}
HTTP/1.1 200
[Asserts]
jsonpath "$.service" == "websocket"
jsonpath "$.status" == "online" 
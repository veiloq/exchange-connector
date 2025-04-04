# Test WebSocket HealthCheck Method
# This file tests the health checking functionality for WebSocket connections

# Since Hurl doesn't directly support WebSocket connections,
# we simulate health checks via a REST API endpoint

# 1. Check overall WebSocket service health
GET {{api_base_url}}/health
HTTP/1.1 200
[Asserts]
jsonpath "$.status" == "ok"
jsonpath "$.services.websocket" == "healthy"

# 2. Check specific connection health
GET {{api_base_url}}/connections/{{connection_id}}/health
HTTP/1.1 200
[Asserts]
jsonpath "$.connection_id" == "{{connection_id}}"
jsonpath "$.status" == "healthy"
jsonpath "$.metrics.message_count" exists
jsonpath "$.metrics.connected_time" exists

# 3. Test connection with problematic health status
GET {{api_base_url}}/connections/problematic/health
HTTP/1.1 200
[Asserts]
jsonpath "$.status" == "degraded"
jsonpath "$.reason" exists 
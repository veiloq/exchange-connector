# Test WebSocket GetMetrics Method
# This file tests retrieving metrics from a WebSocket connection

# Since Hurl doesn't directly support WebSocket connections,
# we simulate metrics retrieval via a REST API endpoint

# 1. Get metrics for a specific connection
GET {{api_base_url}}/connections/{{connection_id}}/metrics
HTTP/1.1 200
[Asserts]
jsonpath "$.connection_id" == "{{connection_id}}"
jsonpath "$.metrics.message_count" exists
jsonpath "$.metrics.reconnect_count" exists
jsonpath "$.metrics.error_count" exists
jsonpath "$.metrics.connected_time" exists

# 2. Get aggregated metrics for all connections
GET {{api_base_url}}/metrics
[QueryStringParams]
filter: websocket

HTTP/1.1 200
[Asserts]
jsonpath "$.total_connections" exists
jsonpath "$.active_connections" exists
jsonpath "$.total_messages" exists
jsonpath "$.connections[0].id" exists 
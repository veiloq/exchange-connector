# Test WebSocket Close Method
# This file tests closing a WebSocket connection

# Since Hurl doesn't directly support WebSocket connections,
# we simulate connection closing via a REST API endpoint

# 1. First, check that the connection exists
GET {{api_base_url}}/connections/{{connection_id}}
HTTP/1.1 200
[Asserts]
jsonpath "$.connection_id" == "{{connection_id}}"
jsonpath "$.status" == "connected"

# 2. Close the connection
POST {{api_base_url}}/connections/{{connection_id}}/close
Content-Type: application/json
{
  "reason": "client_requested"
}

HTTP/1.1 200
[Asserts]
jsonpath "$.status" == "closed"

# 3. Verify the connection is closed
GET {{api_base_url}}/connections/{{connection_id}}
HTTP/1.1 200
[Asserts]
jsonpath "$.status" == "closed" 
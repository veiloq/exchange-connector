# Test WebSocket Subscribe Method
# This file tests subscription to a WebSocket topic

# Since Hurl doesn't directly support WebSocket subscriptions,
# we simulate it by using the REST API subscription endpoint

# 1. Create a subscription via REST API
POST {{api_base_url}}/subscriptions
Content-Type: application/json
{
  "topic": "orderBookL2_25.BTCUSDT",
  "callback_url": "{{callback_url}}",
  "expiry_seconds": 3600
}

HTTP/1.1 201
[Asserts]
jsonpath "$.subscription_id" exists
jsonpath "$.topic" == "orderBookL2_25.BTCUSDT"
jsonpath "$.status" == "active"

# 2. Verify subscription was created successfully
GET {{api_base_url}}/subscriptions/{{subscription_id}}
HTTP/1.1 200
[Asserts]
jsonpath "$.topic" == "orderBookL2_25.BTCUSDT"
jsonpath "$.status" == "active" 
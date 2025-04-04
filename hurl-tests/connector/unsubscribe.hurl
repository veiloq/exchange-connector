# Test WebSocket Unsubscribe Method
# This file tests unsubscribing from a WebSocket topic

# Since Hurl doesn't directly support WebSocket operations,
# we simulate unsubscribing via a REST API endpoint

# 1. First, we need a subscription to remove (this might be created in a previous test)
# Get existing subscription to confirm it exists
GET {{api_base_url}}/subscriptions/{{subscription_id}}
HTTP/1.1 200
[Asserts]
jsonpath "$.subscription_id" == "{{subscription_id}}"
jsonpath "$.status" == "active"

# 2. Unsubscribe from the topic
DELETE {{api_base_url}}/subscriptions/{{subscription_id}}
HTTP/1.1 200
[Asserts]
jsonpath "$.status" == "deleted"

# 3. Verify the subscription was removed
GET {{api_base_url}}/subscriptions/{{subscription_id}}
HTTP/1.1 404 
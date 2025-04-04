# Test WebSocket Send Method
# This file tests sending messages over a WebSocket connection

# Since Hurl doesn't directly support WebSocket connections,
# we simulate sending messages using a REST API endpoint

# Send a message that would normally go through WebSocket
POST {{api_base_url}}/messages
Content-Type: application/json
{
  "topic": "orderBookL2_25.BTCUSDT",
  "message_type": "subscribe",
  "data": {
    "symbol": "BTCUSDT",
    "interval": "1m"
  }
}

HTTP/1.1 202
[Asserts]
jsonpath "$.message_id" exists
jsonpath "$.status" == "accepted"

# Check message delivery status
GET {{api_base_url}}/messages/{{message_id}}
HTTP/1.1 200
[Asserts]
jsonpath "$.status" == "delivered"
jsonpath "$.timestamp" exists 
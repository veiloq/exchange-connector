# Bybit WebSocket API Test
# This file provides examples for testing Bybit WebSocket API endpoints

# Environment variables:
# - BYBIT_API_KEY: Your Bybit API key
# - BYBIT_API_SECRET: Your Bybit API secret

# 1. Check Bybit API Server Status
GET https://api.bybit.com/v5/market/time
HTTP/1.1 200
[Asserts]
jsonpath "$.retCode" == 0
jsonpath "$.result.timeSecond" exists

# 2. Authenticate with Bybit API (required for some WebSocket operations)
POST https://api.bybit.com/v5/market/instruments-info
Content-Type: application/json
X-BAPI-API-KEY: {{BYBIT_API_KEY}}
X-BAPI-TIMESTAMP: {{timestamp}}
X-BAPI-SIGN: {{signature}}
{
  "category": "spot",
  "symbol": "BTCUSDT"
}

HTTP/1.1 200
[Asserts]
jsonpath "$.retCode" == 0
jsonpath "$.result.list" exists

# 3. Get Kline data (which you might subscribe to via WebSocket)
GET https://api.bybit.com/v5/market/kline
[QueryStringParams]
category: spot
symbol: BTCUSDT
interval: 1
limit: 2

HTTP/1.1 200
[Asserts]
jsonpath "$.retCode" == 0
jsonpath "$.result.list" count == 2
jsonpath "$.result.category" == "spot" 
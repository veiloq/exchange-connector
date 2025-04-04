# Binance WebSocket API Test
# This file provides examples for testing Binance WebSocket API endpoints

# Environment variables:
# - BINANCE_API_KEY: Your Binance API key
# - BINANCE_API_SECRET: Your Binance API secret

# 1. Check Binance API Server Status
GET https://api.binance.com/api/v3/ping
HTTP/1.1 200
[Asserts]
body == "{}"

# 2. Get Server Time (useful for signature generation)
GET https://api.binance.com/api/v3/time
HTTP/1.1 200
[Asserts]
jsonpath "$.serverTime" exists

# 3. Get Exchange Information
GET https://api.binance.com/api/v3/exchangeInfo
[QueryStringParams]
symbol: BTCUSDT

HTTP/1.1 200
[Asserts]
jsonpath "$.symbols[0].symbol" == "BTCUSDT"
jsonpath "$.symbols[0].status" == "TRADING"

# 4. Get Recent Trades (data you might subscribe to via WebSocket)
GET https://api.binance.com/api/v3/trades
[QueryStringParams]
symbol: BTCUSDT
limit: 5

HTTP/1.1 200
[Asserts]
jsonpath "$" count == 5
jsonpath "$[0].symbol" == "BTCUSDT"

# 5. Get Kline/Candlestick data (commonly subscribed to in WebSocket)
GET https://api.binance.com/api/v3/klines
[QueryStringParams]
symbol: BTCUSDT
interval: 1m
limit: 2

HTTP/1.1 200
[Asserts]
jsonpath "$" count == 2 
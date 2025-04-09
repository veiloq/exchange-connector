package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/veiloq/exchange-connector/pkg/exchanges/interfaces"
	"github.com/veiloq/exchange-connector/pkg/logging"
	"github.com/veiloq/exchange-connector/pkg/websocket"
)

// Supported time intervals for Bybit candles
var supportedIntervals = map[string]bool{
	"1m":  true,
	"3m":  true,
	"5m":  true,
	"15m": true,
	"30m": true,
	"1h":  true,
	"2h":  true,
	"4h":  true,
	"6h":  true,
	"12h": true,
	"1d":  true,
	"1w":  true,
	"1M":  true,
}

// Connector implements the exchange connector for Bybit API
type Connector struct {
	options *interfaces.ExchangeOptions
	ws      websocket.WSConnector
	logger  logging.Logger

	connected     bool
	subscriptions map[string]struct{}
	instruments   map[string]BybitInstrumentInfo
}

// NewConnector creates a new Bybit exchange connector
func NewConnector(options *interfaces.ExchangeOptions) *Connector {
	if options == nil {
		options = interfaces.NewExchangeOptions()
	}

	// TODO check documentation and build convenient URL buildler
	wsBaseURL := "wss://stream.bybit.com/v5/public/spot"
	if options.BaseURL != "" {
		wsBaseURL = options.BaseURL
	}

	logger := logging.NewLogger()

	// TODO move to a helper module
	if options.LogLevel != "" {
		switch options.LogLevel {
		case "debug":
			logger.SetLevel(logging.DEBUG)
		case "info":
			logger.SetLevel(logging.INFO)
		case "warn":
			logger.SetLevel(logging.WARN)
		case "error":
			logger.SetLevel(logging.ERROR)
		}
	}

	heartbeatInterval := 20 * time.Second
	if options.WSHeartbeatInterval > 0 {
		heartbeatInterval = options.WSHeartbeatInterval
	}

	reconnectInterval := 5 * time.Second
	if options.WSReconnectInterval > 0 {
		reconnectInterval = options.WSReconnectInterval
	}

	return &Connector{
		options: options,
		ws: websocket.NewConnector(websocket.Config{
			URL:               wsBaseURL,
			HeartbeatInterval: heartbeatInterval,
			ReconnectInterval: reconnectInterval,
			MaxRetries:        3,
		}),
		logger:        logger,
		connected:     false,
		subscriptions: make(map[string]struct{}),
		instruments:   make(map[string]BybitInstrumentInfo),
	}
}

// Connect establishes a connection to the Bybit WebSocket API
func (c *Connector) Connect(ctx context.Context) error {
	c.logger.Info("connecting to Bybit WebSocket API",
		logging.String("url", c.ws.GetConfig().URL))

	// Fetch instruments info first
	// Fetch instruments info first
	if err := c.fetchInstrumentsInfo(ctx); err != nil {
		c.logger.Error("failed to fetch initial instruments info", logging.Error(err))
		// Return the error directly from fetchInstrumentsInfo, as it's already descriptive
		return err
	}

	if err := c.ws.Connect(ctx); err != nil {
		c.logger.Error("failed to connect to Bybit", logging.Error(err))
		return fmt.Errorf("failed to connect to Bybit WebSocket API: %w", err)
	}

	c.connected = true
	c.logger.Info("successfully connected to Bybit WebSocket API")
	return nil
}

// Close properly terminates the connection
func (c *Connector) Close() error {
	if !c.connected {
		return interfaces.ErrNotConnected
	}

	c.logger.Info("closing connection to Bybit WebSocket API")
	err := c.ws.Close()
	c.connected = false
	c.subscriptions = make(map[string]struct{})

	if err != nil {
		c.logger.Error("error closing WebSocket connection", logging.Error(err))
		return fmt.Errorf("error closing Bybit WebSocket connection: %w", err)
	}

	c.logger.Info("successfully closed connection to Bybit WebSocket API")
	return nil
}

// isValidSymbol checks if a trading pair symbol is valid
func (c *Connector) isValidSymbol(symbol string) bool {
	if len(c.instruments) == 0 {
		c.logger.Warn("instruments map is empty, falling back to basic symbol validation")
		return symbol != "" && len(symbol) >= 5
	}
	_, ok := c.instruments[symbol]
	return ok
}

// isValidInterval checks if a time interval is supported by Bybit
func (c *Connector) isValidInterval(interval string) bool {
	// Check the map directly using the standard interval format ("1m", "1h", etc.)
	_, ok := supportedIntervals[interval]
	return ok
}

// validateTimeRange checks if a time range is valid
func (c *Connector) validateTimeRange(startTime, endTime time.Time) error {
	if startTime.IsZero() || endTime.IsZero() {
		return interfaces.ErrInvalidTimeRange
	}
	if endTime.Before(startTime) {
		return interfaces.ErrInvalidTimeRange
	}
	return nil
}

// ---------------------------------------------------------
// DRY Helper: doBybitGet
// ---------------------------------------------------------

func (c *Connector) doBybitGet(ctx context.Context, path string, params url.Values) ([]byte, int, error) {
	baseURL, _ := url.Parse(c.options.RestURL)
	if baseURL == nil || baseURL.String() == "" {
		baseURL, _ = url.Parse("https://api.bybit.com")
	}
	baseURL.Path = path
	baseURL.RawQuery = params.Encode()

	c.logger.Debug("constructed Bybit API URL", logging.String("url", baseURL.String()))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL.String(), nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create Bybit API request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to execute Bybit API request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read Bybit API response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("Bybit API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	return body, resp.StatusCode, nil
}

// ---------------------------------------------------------
// DRY Helper: checkBybitRetCode
// ---------------------------------------------------------

type BybitBaseResponse struct {
	RetCode    int             `json:"retCode"`
	RetMsg     string          `json:"retMsg"`
	RetExtInfo json.RawMessage `json:"retExtInfo"`
	Time       int64           `json:"time"`
}

func checkBybitRetCode(r BybitBaseResponse) error {
	if r.RetCode != 0 {
		return fmt.Errorf("Bybit API error: code %d, message: %s", r.RetCode, r.RetMsg)
	}
	return nil
}

func parseBybitResponse[T interface{ GetBase() BybitBaseResponse }](body []byte) (*T, error) {
	var resp T
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse Bybit response JSON: %w", err)
	}
	base := resp.GetBase()
	if err := checkBybitRetCode(base); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Each response struct implements this so parseBybitResponse can fetch the base part.
func (r BybitKlineResponse) GetBase() BybitBaseResponse {
	return BybitBaseResponse{
		RetCode:    r.RetCode,
		RetMsg:     r.RetMsg,
		RetExtInfo: r.RetExtInfo,
		Time:       r.Time,
	}
}
func (r BybitOrderBookResponse) GetBase() BybitBaseResponse {
	return BybitBaseResponse{
		RetCode:    r.RetCode,
		RetMsg:     r.RetMsg,
		RetExtInfo: r.RetExtInfo,
		Time:       r.Time,
	}
}
func (r BybitInstrumentsInfoResponse) GetBase() BybitBaseResponse {
	return BybitBaseResponse{
		RetCode:    r.RetCode,
		RetMsg:     r.RetMsg,
		RetExtInfo: r.RetExtInfo,
		Time:       r.Time,
	}
}

// ---------------------------------------------------------
// Small parse helpers to reduce repetition
// ---------------------------------------------------------

// parseFloatWithLog tries to parse a float; logs a warning on failure.
func parseFloatWithLog(logger logging.Logger, val string, fieldName string) (float64, bool) {
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		logger.Warn(fmt.Sprintf("failed to parse %s", fieldName),
			logging.String("value", val),
			logging.Error(err))
		return 0, false
	}
	return f, true
}

// parseKlineRow expects something like [timestamp, open, high, low, close, volume, ...]
func parseKlineRow(logger logging.Logger, row []string) (time.Time, float64, float64, float64, float64, float64, bool) {
	if len(row) < 7 {
		logger.Warn("incomplete kline data point", logging.String("data", fmt.Sprintf("%v", row)))
		return time.Time{}, 0, 0, 0, 0, 0, false
	}

	startTimeMs, err := strconv.ParseInt(row[0], 10, 64)
	if err != nil {
		logger.Warn("failed to parse timestamp",
			logging.String("value", row[0]),
			logging.Error(err))
		return time.Time{}, 0, 0, 0, 0, 0, false
	}

	open, ok1 := parseFloatWithLog(logger, row[1], "open price")
	high, ok2 := parseFloatWithLog(logger, row[2], "high price")
	low, ok3 := parseFloatWithLog(logger, row[3], "low price")
	closePrice, ok4 := parseFloatWithLog(logger, row[4], "close price")
	volume, ok5 := parseFloatWithLog(logger, row[5], "volume")

	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 {
		// Already logged, skip this row
		return time.Time{}, 0, 0, 0, 0, 0, false
	}

	return time.UnixMilli(startTimeMs), open, high, low, closePrice, volume, true
}

// parseOrderBookRow expects something like [price, quantity]
func parseOrderBookRow(logger logging.Logger, row []string, rowType string) (float64, float64, bool) {
	if len(row) != 2 {
		logger.Warn(fmt.Sprintf("invalid %s format", rowType),
			logging.String("row_data", fmt.Sprintf("%v", row)))
		return 0, 0, false
	}

	price, ok1 := parseFloatWithLog(logger, row[0], rowType+" price")
	quantity, ok2 := parseFloatWithLog(logger, row[1], rowType+" quantity")
	if !ok1 || !ok2 {
		return 0, 0, false
	}

	return price, quantity, true
}

// ---------------------------------------------------------
// GetCandles
// ---------------------------------------------------------

func (c *Connector) GetCandles(ctx context.Context, req interfaces.CandleRequest) ([]interfaces.Candle, error) {
	if !c.connected { // Check connection status first
		return nil, interfaces.ErrNotConnected
	}
	if !c.isValidSymbol(req.Symbol) {
		return nil, interfaces.ErrInvalidSymbol
	}
	if !c.isValidInterval(req.Interval) {
		return nil, interfaces.ErrInvalidInterval
	}
	if err := c.validateTimeRange(req.StartTime, req.EndTime); err != nil {
		return nil, err
	}

	if req.Limit <= 0 {
		req.Limit = 100
	} else if req.Limit > 1000 {
		req.Limit = 1000
	}

	c.logger.Debug("fetching candles from Bybit API",
		logging.String("symbol", req.Symbol),
		logging.String("interval", req.Interval),
		logging.String("start", req.StartTime.Format(time.RFC3339)),
		logging.String("end", req.EndTime.Format(time.RFC3339)),
		logging.Int("limit", req.Limit),
	)

	params := url.Values{}
	params.Add("category", "spot")
	params.Add("symbol", req.Symbol)                             // Add symbol to params
	params.Add("interval", convertIntervalToBybit(req.Interval)) // Convert and add interval
	params.Add("start", strconv.FormatInt(req.StartTime.UnixMilli(), 10))
	params.Add("end", strconv.FormatInt(req.EndTime.UnixMilli(), 10))
	params.Add("limit", strconv.Itoa(req.Limit))

	body, _, err := c.doBybitGet(ctx, "/v5/market/kline", params)
	if err != nil {
		c.logger.Error("failed to get candles from Bybit API", logging.Error(err))
		return nil, err
	}

	bybitResp, err := parseBybitResponse[BybitKlineResponse](body)
	if err != nil {
		c.logger.Error("failed to parse Bybit Kline response", logging.Error(err))
		return nil, err
	}

	candles := make([]interfaces.Candle, 0, len(bybitResp.Result.List)) // Preallocate slice
	for _, row := range bybitResp.Result.List {
		st, o, h, l, closeP, vol, ok := parseKlineRow(c.logger, row)
		if !ok {
			continue
		}
		candles = append(candles, interfaces.Candle{
			Symbol:    req.Symbol,
			StartTime: st,
			Open:      o,
			High:      h,
			Low:       l,
			Close:     closeP,
			Volume:    vol,
		})
	}

	c.logger.Info("successfully fetched candles from Bybit API",
		logging.String("symbol", req.Symbol),
		logging.Int("count", len(candles)))

	return candles, nil
}

// ---------------------------------------------------------
// GetOrderBook
// ---------------------------------------------------------

func (c *Connector) GetOrderBook(ctx context.Context, symbol string, depth int) (*interfaces.OrderBook, error) {
	if !c.connected { // Check connection status first
		return nil, interfaces.ErrNotConnected
	}
	if !c.isValidSymbol(symbol) {
		return nil, interfaces.ErrInvalidSymbol
	}
	bybitDepth := 50
	if depth <= 1 {
		bybitDepth = 1
	} else if depth <= 50 {
		bybitDepth = 50
	} else {
		bybitDepth = 200
	}
	c.logger.Debug("adjusted order book depth",
		logging.Int("requested_depth", depth),
		logging.Int("api_depth", bybitDepth),
	)

	params := url.Values{}
	params.Add("category", "spot")
	params.Add("symbol", symbol)                  // Add symbol to params
	params.Add("limit", strconv.Itoa(bybitDepth)) // Add depth limit

	body, _, err := c.doBybitGet(ctx, "/v5/market/orderbook", params)
	if err != nil {
		c.logger.Error("failed to get order book", logging.Error(err))
		return nil, err
	}

	bybitResp, err := parseBybitResponse[BybitOrderBookResponse](body)
	if err != nil {
		c.logger.Error("failed to parse Bybit OrderBook response", logging.Error(err))
		return nil, err
	}

	orderBook := &interfaces.OrderBook{
		Symbol: bybitResp.Result.Symbol,
		Bids:   make([]interfaces.OrderBookLevel, 0, len(bybitResp.Result.Bids)),
		Asks:   make([]interfaces.OrderBookLevel, 0, len(bybitResp.Result.Asks)),
	}

	for _, bid := range bybitResp.Result.Bids {
		price, quantity, ok := parseOrderBookRow(c.logger, bid, "bid")
		if !ok {
			continue
		}
		orderBook.Bids = append(orderBook.Bids, interfaces.OrderBookLevel{
			Price:    price,
			Quantity: quantity,
		})
	}
	for _, ask := range bybitResp.Result.Asks {
		price, quantity, ok := parseOrderBookRow(c.logger, ask, "ask")
		if !ok {
			continue
		}
		orderBook.Asks = append(orderBook.Asks, interfaces.OrderBookLevel{
			Price:    price,
			Quantity: quantity,
		})
	}

	c.logger.Info("successfully fetched order book from Bybit API",
		logging.String("symbol", orderBook.Symbol),
		logging.Int("bid_count", len(orderBook.Bids)),
		logging.Int("ask_count", len(orderBook.Asks)))

	return orderBook, nil
}

// ---------------------------------------------------------
// SubscribeCandles
// ---------------------------------------------------------

// SubscribeCandles subscribes to the candlestick stream for a given symbol and interval.
func (c *Connector) SubscribeCandles(symbol string, interval string) error {
	if !c.connected { // Check connection status first
		return interfaces.ErrNotConnected
	}

	if !c.isValidInterval(interval) {
		return interfaces.ErrInvalidInterval
	}

	// Bybit uses specific topic formats, e.g., kline.{interval}.{symbol}
	bybitInterval := convertIntervalToBybit(interval)
	topic := fmt.Sprintf("kline.%s.%s", bybitInterval, symbol)

	// Check if already subscribed (optional, depends on desired behavior)
	if _, exists := c.subscriptions[topic]; exists {
		c.logger.Warn("already subscribed to topic", logging.String("topic", topic))
		return nil // Or return a specific error like ErrAlreadySubscribed
	}

	c.logger.Info("subscribing to Bybit topic", logging.String("topic", topic))
	// The actual subscription logic depends on the websocket.WSConnector implementation.
	// Assuming it takes the topic and a message handler (which might be nil for public streams
	// or handled internally by the WSConnector).
	err := c.ws.Subscribe(topic, nil) // Pass nil or an appropriate handler
	if err != nil {
		c.logger.Error("failed to subscribe via WebSocket", logging.String("topic", topic), logging.Error(err))
		return fmt.Errorf("failed to subscribe to Bybit topic %s: %w", topic, err)
	}

	// Record the subscription internally
	c.subscriptions[topic] = struct{}{}
	c.logger.Info("successfully subscribed to Bybit topic", logging.String("topic", topic))
	return nil
}

// ---------------------------------------------------------
// UnsubscribeCandles
// ---------------------------------------------------------

// UnsubscribeCandles unsubscribes from the candlestick stream for a given symbol and interval.
func (c *Connector) UnsubscribeCandles(symbol string, interval string) error {
	if !c.connected { // Check connection status first
		return interfaces.ErrNotConnected
	}

	// Validate interval (optional but good practice)
	if !c.isValidInterval(interval) {
		return interfaces.ErrInvalidInterval
	}

	bybitInterval := convertIntervalToBybit(interval)
	topic := fmt.Sprintf("kline.%s.%s", bybitInterval, symbol)

	// Check if actually subscribed
	if _, exists := c.subscriptions[topic]; !exists {
		return fmt.Errorf("not subscribed to topic: %s", topic) // Return error if not subscribed
	}

	c.logger.Info("unsubscribing from Bybit topic", logging.String("topic", topic))
	err := c.ws.Unsubscribe(topic)
	if err != nil {
		c.logger.Error("failed to unsubscribe via WebSocket", logging.String("topic", topic), logging.Error(err))
		// Decide if the internal state should be cleaned up even if WS call fails
		// delete(c.subscriptions, topic) // Optional: remove even on error?
		return fmt.Errorf("failed to unsubscribe from Bybit topic %s: %w", topic, err)
	}

	// Remove the subscription from internal tracking
	delete(c.subscriptions, topic)
	c.logger.Info("successfully unsubscribed from Bybit topic", logging.String("topic", topic))
	return nil
}

// ---------------------------------------------------------
// fetchInstrumentsInfo
// ---------------------------------------------------------

func (c *Connector) fetchInstrumentsInfo(ctx context.Context) error {
	c.logger.Info("fetching instruments info from Bybit API")

	params := url.Values{}
	params.Add("category", "spot")

	body, _, err := c.doBybitGet(ctx, "/v5/market/instruments-info", params)
	if err != nil {
		return fmt.Errorf("failed to fetch Bybit instruments info: %w", err)
	}

	bybitResp, err := parseBybitResponse[BybitInstrumentsInfoResponse](body)
	if err != nil {
		return fmt.Errorf("failed to parse Bybit instruments info JSON: %w", err)
	}

	newInstruments := make(map[string]BybitInstrumentInfo)
	for _, instrument := range bybitResp.Result.List {
		if instrument.Status == "Trading" {
			newInstruments[instrument.Symbol] = instrument
		}
	}
	c.instruments = newInstruments // Update the connector's instruments map

	c.logger.Info("successfully fetched and updated instruments info",
		logging.Int("count", len(c.instruments)))
	return nil
}

// Helper function to convert interval string
func convertIntervalToBybit(interval string) string {
	switch interval {
	case "1m":
		return "1"
	case "3m":
		return "3"
	case "5m":
		return "5"
	case "15m":
		return "15"
	case "30m":
		return "30"
	case "1h":
		return "60"
	case "2h":
		return "120"
	case "4h":
		return "240"
	case "6h":
		return "360"
	case "12h":
		return "720"
	case "1d":
		return "D"
	case "1w":
		return "W"
	case "1M":
		return "M"
	default:
		return interval
	}
}

// ---------------------------------------------------------
// Bybit API Response Structs
// ---------------------------------------------------------

type BybitKlineResponse struct {
	BybitBaseResponse
	Result BybitKlineResult `json:"result"`
}

type BybitKlineResult struct {
	Category string     `json:"category"`
	Symbol   string     `json:"symbol"`
	List     [][]string `json:"list"`
}

type BybitOrderBookResponse struct {
	BybitBaseResponse
	Result BybitOrderBookResult `json:"result"`
}

type BybitOrderBookResult struct {
	Symbol    string     `json:"s"`
	Bids      [][]string `json:"b"`
	Asks      [][]string `json:"a"`
	Timestamp int64      `json:"ts"`
	UpdateID  int64      `json:"u"`
}

type BybitInstrumentsInfoResponse struct {
	BybitBaseResponse
	Result BybitInstrumentsInfoResult `json:"result"`
}

type BybitInstrumentsInfoResult struct {
	Category       string                `json:"category"`
	List           []BybitInstrumentInfo `json:"list"`
	NextPageCursor string                `json:"nextPageCursor"`
}

type BybitInstrumentInfo struct {
	Symbol        string             `json:"symbol"`
	BaseCoin      string             `json:"baseCoin"`
	QuoteCoin     string             `json:"quoteCoin"`
	Innovation    string             `json:"innovation"`
	Status        string             `json:"status"`
	LotSizeFilter BybitLotSizeFilter `json:"lotSizeFilter"`
	PriceFilter   BybitPriceFilter   `json:"priceFilter"`
}

type BybitLotSizeFilter struct {
	MaxOrderQty   string `json:"maxOrderQty"`
	MinOrderQty   string `json:"minOrderQty"`
	QtyStep       string `json:"qtyStep"`
	BasePrecision string `json:"basePrecision"`
}

type BybitPriceFilter struct {
	TickSize string `json:"tickSize"`
	MinPrice string `json:"minPrice"`
	MaxPrice string `json:"maxPrice"`
}

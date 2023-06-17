package types

const (
	// enums
	BUY  OrderType = "buy"
	SELL OrderType = "sell"
)

type Book struct {
	Data struct {
		Bids [][]string `json:"bids"`
		Asks [][]string `json:"asks"`
	} `json:"data"`
}

type BaseResponse struct {
	Channel string `json:"channel"`
	Event   string `json:"event"`
}

type HttpBook struct {
	Bids [][]string `json:"bids"`
	Asks [][]string `json:"asks"`
}

type Top1Book struct {
	Pair      string
	BidPrice  float64
	BidAmount float64
	AskPrice  float64
	AskAmount float64
}

type OrderType string

type Config struct {
	Pairs []string             `json:"pairs"`
	Types map[string]OrderType `json:"types"`
}

type CurrencyPair struct {
	From string
	To   string
}

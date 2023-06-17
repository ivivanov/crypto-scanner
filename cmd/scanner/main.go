package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"

	crypto "github.com/ivivanov/crypto-scanner/types"
)

const (
	WS_ENDPOINT      = "ws.bitstamp.net"
	MIN_PNL          = 0
	FX_PATH          = "fx.json"
	PAIR_CYCLES_PATH = "../find-cycles/pair-cycles.json"
	CONFIG_PATH      = "../find-cycles/config.json"
)

var (
	// fmt print in red
	red = color.New(color.FgRed)
)

type App struct {
	config          map[string]crypto.Config
	fxPairs         []string
	pairToCycles    map[string][]string
	pairTop1Book    map[string]crypto.Top1Book
	takerFeeReduced float64
	takerFee        float64
	wsConn          *websocket.Conn

	updateBookC chan crypto.Top1Book
	interruptC  chan os.Signal
}

func (app *App) Subscribe() error {
	for _, c := range app.config {
		for _, pair := range c.Pairs {
			msg := []byte(fmt.Sprintf(`{"event":"bts:subscribe","data":{"channel":"order_book_%v"}}`, pair))
			app.wsConn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			err := app.wsConn.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (app *App) HandleMsgHttp(resp *http.Response, pair string) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return
	}

	book := &crypto.HttpBook{}
	err = json.Unmarshal(body, book)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("%v book update: %v", pair, book)

	bidPrice, _ := strconv.ParseFloat(book.Bids[0][0], 64)
	bidAmount, _ := strconv.ParseFloat(book.Bids[0][1], 64)
	askPrice, _ := strconv.ParseFloat(book.Asks[0][0], 64)
	askAmount, _ := strconv.ParseFloat(book.Asks[0][1], 64)

	app.updateBookC <- crypto.Top1Book{
		Pair:      pair,
		BidPrice:  bidPrice,
		BidAmount: bidAmount,
		AskPrice:  askPrice,
		AskAmount: askAmount,
	}
}

func (app *App) HandleMsgWS(raw []byte) {
	baseMsg := crypto.BaseResponse{}
	err := json.Unmarshal(raw, &baseMsg)
	if err != nil {
		log.Fatalf("unmarshal: %v", err)
	}

	switch baseMsg.Event {
	case "bts:subscription_succeeded":
		fmt.Println(string(raw))
	case "data":
		book := &crypto.Book{}
		err = json.Unmarshal(raw, book)
		if err != nil {
			fmt.Println(err)
			return
		}

		bidPrice, _ := strconv.ParseFloat(book.Data.Bids[0][0], 64)
		bidAmount, _ := strconv.ParseFloat(book.Data.Bids[0][1], 64)
		askPrice, _ := strconv.ParseFloat(book.Data.Asks[0][0], 64)
		askAmount, _ := strconv.ParseFloat(book.Data.Asks[0][1], 64)

		app.updateBookC <- crypto.Top1Book{
			Pair:      strings.TrimPrefix(baseMsg.Channel, "order_book_"),
			BidPrice:  bidPrice,
			BidAmount: bidAmount,
			AskPrice:  askPrice,
			AskAmount: askAmount,
		}
	default:
		fmt.Println(string(raw))
	}
}

func (app *App) GetFee(pair string) float64 {
	for _, v := range app.fxPairs {
		if pair == v {
			return app.takerFeeReduced / 100
		}
	}

	return app.takerFee / 100
}

func (app *App) CalcTriangularArb(cycle string) float64 {
	startAmount := 1.0
	config := app.config[cycle]
	trade1 := app.CalcTrade(cycle, float64(startAmount), app.GetFee(config.Pairs[0]), app.pairTop1Book[config.Pairs[0]])
	trade2 := app.CalcTrade(cycle, trade1, app.GetFee(config.Pairs[1]), app.pairTop1Book[config.Pairs[1]])
	trade3 := app.CalcTrade(cycle, trade2, app.GetFee(config.Pairs[2]), app.pairTop1Book[config.Pairs[2]])
	pnl := ((trade3 - startAmount) / startAmount) * 100

	return pnl
}

func (app *App) CalcTrade(cycle string, amount, fee float64, pair crypto.Top1Book) float64 {
	afterFee := amount - amount*fee
	config := app.config[cycle]

	switch config.Types[pair.Pair] {
	case crypto.BUY:
		return afterFee / pair.AskPrice
	case crypto.SELL:
		return afterFee * pair.BidPrice
	default:
		return 0 // todo err
	}
}

func NewApp() (*App, error) {
	err := godotenv.Load()
	if err != nil {
		return nil, fmt.Errorf("load .env: %v", err)
	}

	config, err := loadConfig()
	if err != nil {
		return nil, fmt.Errorf("load config: %v", err)
	}

	fxPairs, err := loadFxPairs()
	if err != nil {
		return nil, fmt.Errorf("load fx pairs: %v", err)
	}

	pairToCycles, err := loadPairCycles()
	if err != nil {
		return nil, fmt.Errorf("load pair cycles: %v", err)
	}

	takerFeeReduced, err := strconv.ParseFloat(os.Getenv("TAKER_FEE_REDUCED"), 64)
	if err != nil {
		return nil, fmt.Errorf("load fees: %v", err)
	}

	takerFee, err := strconv.ParseFloat(os.Getenv("TAKER_FEE"), 64)
	if err != nil {
		return nil, fmt.Errorf("load fees: %v", err)
	}

	wsUrl := url.URL{Scheme: "wss", Host: WS_ENDPOINT}
	fmt.Printf("connecting to %s\n", wsUrl.String())
	wsConn, httpResp, err := websocket.DefaultDialer.Dial(wsUrl.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("dial ws: %v", err)
	}

	fmt.Printf("dial status: %v\n", httpResp.StatusCode)

	return &App{
		config:          config,
		fxPairs:         fxPairs,
		pairToCycles:    pairToCycles,
		pairTop1Book:    make(map[string]crypto.Top1Book),
		takerFeeReduced: takerFeeReduced,
		takerFee:        takerFee,
		wsConn:          wsConn,

		updateBookC: make(chan crypto.Top1Book),
		interruptC:  make(chan os.Signal, 1),
	}, nil
}

func main() {
	app, err := NewApp()
	if err != nil {
		fmt.Println(err)
		return
	}

	defer close(app.updateBookC)
	defer close(app.interruptC)
	defer app.wsConn.Close()
	signal.Notify(app.interruptC, os.Interrupt)

	// async: init order book
	go func() {
		app.InitOrderBooks()
	}()

	// thread: read ws msgs
	go func() {
		for {
			_, msg, err := app.wsConn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					fmt.Printf("error: %v", err)
				}

				fmt.Printf("read error: %v", err)
				break
			}

			app.HandleMsgWS(msg)
		}
	}()

	// subscribe
	go func() {
		err = app.Subscribe()
		if err != nil {
			fmt.Printf("write error: %v", err)
			return
		}
	}()

	// block the main thread
	// i := 0
	for {
		select {
		case update := <-app.updateBookC:
			app.pairTop1Book[update.Pair] = update
			for _, v := range app.pairToCycles[update.Pair] {
				pnl := app.CalcTriangularArb(v)
				if pnl > MIN_PNL {
					log.Println(red.Sprintf("%v %v", v, pnl))
				}

				// if i < 5 { // sanity check that initial https request have initialized the order books
				// 	fmt.Println(pnl)
				// 	i++
				// } else if pnl > MIN_PNL {
				// 	log.Println(red.Sprintf("%v", pnl)) // log adds datetime
				// }
			}

		case <-app.interruptC:
			log.Println("interrupt triggered by user")

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := app.wsConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				return
			}

			<-time.After(time.Second)

			return
		}
	}
}

func loadConfig() (map[string]crypto.Config, error) {
	pairsJson, err := os.ReadFile(CONFIG_PATH)
	if err != nil {
		return nil, err
	}

	config := make(map[string]crypto.Config)
	err = json.Unmarshal(pairsJson, &config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func loadFxPairs() ([]string, error) {
	fxJson, err := os.ReadFile(FX_PATH)
	if err != nil {
		return nil, err
	}

	var fxPairs []string
	err = json.Unmarshal(fxJson, &fxPairs)
	if err != nil {
		return nil, err
	}

	for i, v := range fxPairs {
		fxPairs[i] = strings.ToLower(v)
	}

	return fxPairs, nil
}

func loadPairCycles() (map[string][]string, error) {
	pairsJson, err := os.ReadFile(PAIR_CYCLES_PATH)
	if err != nil {
		return nil, err
	}

	config := make(map[string][]string)
	err = json.Unmarshal(pairsJson, &config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func (app *App) InitOrderBooks() {
	for pair := range app.pairToCycles {
		resp, err := http.Get(fmt.Sprintf("https://www.bitstamp.net/api/v2/order_book/%v", pair))
		if err != nil {
			fmt.Printf("%v init order book: %v", pair, err)
			continue
		}

		app.HandleMsgHttp(resp, pair)
	}
}

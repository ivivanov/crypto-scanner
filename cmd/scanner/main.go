package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"

	crypto "github.com/ivivanov/crypto-scanner/types"
)

const (
	WS_ENDPOINT = "ws.bitstamp.net"
	MIN_PNL     = 0
)

var (
	config crypto.Config
	fees   crypto.Fees

	pairTop1Book = make(map[string]crypto.Top1Book)
	updateBookC  = make(chan crypto.Top1Book)

	// fmt print in red
	red = color.New(color.FgRed)
)

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Printf("load .env: %v", err)
		return
	}

	err = LoadConfig()
	if err != nil {
		fmt.Printf("load config: %v", err)
		return
	}

	err = LoadFees()
	if err != nil {
		fmt.Printf("load fees: %v", err)
		return
	}

	err = ValidateFeesConfig()
	if err != nil {
		fmt.Printf("missing fees: %v", err)
		return
	}

	defer close(updateBookC)

	wsUrl := url.URL{Scheme: "wss", Host: WS_ENDPOINT}
	fmt.Printf("connecting to %s", wsUrl.String())
	wsConn, httpResp, err := websocket.DefaultDialer.Dial(wsUrl.String(), nil)
	if err != nil {
		fmt.Printf("dial ws: %v", err)
		return
	}

	fmt.Printf("dial status: %v", httpResp.StatusCode)

	// async: init order book
	go func() {
		InitOrderBooks()
	}()

	// thread: read ws msgs
	go func() {
		for {
			_, msg, err := wsConn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					fmt.Printf("error: %v", err)
				}

				fmt.Printf("read error: %v", err)
				break
			}

			HandleMsgWS(msg)
		}
	}()

	// subscribe
	go func() {
		err = Subscribe(wsConn)
		if err != nil {
			fmt.Printf("write error: %v", err)
			return
		}
	}()

	// block the main thread
	i := 0
	for {
		update := <-updateBookC
		pairTop1Book[update.Pair] = update
		pnl := CalcTriangularArb()

		if i < 5 { // sanity check that initial https request have initialized the order books
			fmt.Println(pnl)
			i++
		} else if pnl > MIN_PNL {
			log.Println(red.Sprintf("%v", pnl)) // log adds datetime
		}
	}
}

func Subscribe(wsConn *websocket.Conn) error {
	for _, pair := range config.Pairs {
		msg := []byte(fmt.Sprintf(`{"event":"bts:subscribe","data":{"channel":"order_book_%v"}}`, pair))
		wsConn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		err := wsConn.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			return err
		}
	}

	return nil
}

func CalcTriangularArb() float64 {
	startAmount := 1.0
	trade1 := CalcTrade(float64(startAmount), GetFee(config.Pairs[0]), pairTop1Book[config.Pairs[0]])
	trade2 := CalcTrade(trade1, GetFee(config.Pairs[1]), pairTop1Book[config.Pairs[1]])
	trade3 := CalcTrade(trade2, GetFee(config.Pairs[2]), pairTop1Book[config.Pairs[2]])
	pnl := ((trade3 - startAmount) / startAmount) * 100

	return pnl
}

func CalcTrade(amount, fee float64, pair crypto.Top1Book) float64 {
	afterFee := amount - amount*fee

	switch config.Types[pair.Pair] {
	case crypto.BUY:
		return afterFee / pair.AskPrice
	case crypto.SELL:
		return afterFee * pair.BidPrice
	default:
		return 0 // todo err
	}
}

func GetFee(pair string) float64 {
	return fees.TakerFees[pair] / 100
}

func LoadConfig() error {
	configPath := "../../config.json"
	envConfigPath := os.Getenv("CONFIG_PATH")
	if envConfigPath != "" {
		configPath = envConfigPath
	}

	pairsJson, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var configs map[string]crypto.Config
	err = json.Unmarshal(pairsJson, &configs)
	if err != nil {
		return err
	}

	val, ok := configs[os.Getenv("COMBO")]
	if !ok {
		return fmt.Errorf("invalid config key")
	}

	config = val

	configJson, err := json.MarshalIndent(val, "", "  ")
	if err != nil {
		return err
	}

	fmt.Printf("loaded config: %v", string(configJson))

	return nil
}

func LoadFees() error {
	feesPath := "../../fees.json"
	envConfigPath := os.Getenv("FEES_PATH")
	if envConfigPath != "" {
		feesPath = envConfigPath
	}

	feesJson, err := os.ReadFile(feesPath)
	if err != nil {
		return err
	}

	err = json.Unmarshal(feesJson, &fees)
	if err != nil {
		return err
	}

	return nil
}

func HandleMsgHttp(resp *http.Response, pair string) {

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

	bidPrice, _ := strconv.ParseFloat(book.Bids[0][0], 64)
	bidAmount, _ := strconv.ParseFloat(book.Bids[0][1], 64)
	askPrice, _ := strconv.ParseFloat(book.Asks[0][0], 64)
	askAmount, _ := strconv.ParseFloat(book.Asks[0][1], 64)

	updateBookC <- crypto.Top1Book{
		Pair:      pair,
		BidPrice:  bidPrice,
		BidAmount: bidAmount,
		AskPrice:  askPrice,
		AskAmount: askAmount,
	}
}

func HandleMsgWS(raw []byte) {
	baseMsg := crypto.BaseResponse{}
	err := json.Unmarshal(raw, &baseMsg)
	if err != nil {
		fmt.Println("unmarshal:", err)
		return
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

		updateBookC <- crypto.Top1Book{
			Pair:      strings.TrimPrefix(baseMsg.Channel, "order_book_"),
			BidPrice:  bidPrice,
			BidAmount: bidAmount,
			AskPrice:  askPrice,
			AskAmount: askAmount,
		}
	default:
		fmt.Println(string(raw))
	}

	if err != nil {
		log.Fatal(err)
	}
}

func InitOrderBooks() {
	for _, pair := range config.Pairs {
		resp, err := http.Get(fmt.Sprintf("https://www.bitstamp.net/api/v2/order_book/%v", pair))
		if err != nil {
			fmt.Println(err)
			return
		}

		HandleMsgHttp(resp, pair)
	}
}

func ValidateFeesConfig() error {
	for _, pair := range config.Pairs {
		_, ok := fees.TakerFees[pair]
		if !ok {
			return fmt.Errorf("pair not found in fees.json")
		}
	}

	return nil
}

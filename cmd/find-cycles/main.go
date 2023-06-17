package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"

	crypto "github.com/ivivanov/crypto-scanner/types"
)

type CurrencyPair struct {
	From string
	To   string
}

const (
	PAIRS_JSON  = "pairs.json"
	TICKERS_TXT = "tickers.txt" // comma delimited list of tickers
	CYCLE_LEN   = 3
)

type App struct {
	Tickers []string
	Pairs   [][]string
}

func NewApp() (*App, error) {
	pairs, err := LoadPairs()
	if err != nil {
		return nil, err
	}

	tickers, err := LoadTickers()
	if err != nil {
		return nil, err
	}

	return &App{
		Pairs:   pairs,
		Tickers: tickers,
	}, nil
}

func (app *App) TickerExists(ticker string) bool {
	for _, v := range app.Tickers {
		if v == ticker {
			return true
		}
	}

	return false
}

func (app *App) RestoreTickers(cycle string) (string, []string, error) {
	currencies := strings.Split(cycle, "-->")
	// convert to: eth-->usd-->btc-->eth
	currencies = append(currencies, currencies[0])

	var res []string
	for i := 0; i < len(currencies)-1; i++ {
		c1 := currencies[i]
		c2 := currencies[i+1]
		if app.TickerExists(c1 + c2) {
			res = append(res, c1+c2)
		} else if app.TickerExists(c2 + c1) {
			res = append(res, c2+c1)
		} else {
			return "", nil, fmt.Errorf("Invalid tickers")
		}
	}

	return currencies[0], res, nil
}

func main() {
	app, err := NewApp()
	if err != nil {
		fmt.Printf("Creating app: %v", err)
		return
	}

	graph, err := NewUndirectedGraph(app.Pairs)
	if err != nil {
		fmt.Printf("Creating graph: %v", err)
		return
	}

	cycles := Loading(graph, CYCLE_LEN, findCycles)
	fmt.Printf("Cycles of length %d:\n", CYCLE_LEN)
	res := make(map[string]crypto.Config)

	// build config
	for k, _ := range cycles {
		_, path, err := app.RestoreTickers(k)
		if err != nil {
			fmt.Printf("Restore tickers: %v", err)
			return
		}

		conf, err := NewConfig(k, path)
		if err != nil {
			fmt.Printf("Creating config: %v", err)
			return
		}

		key := strings.ReplaceAll(k, "-->", "-")
		res[key] = *conf
	}

	// convert to json
	configJson, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		fmt.Printf("Creating config: %v", err)
		return
	}

	// write json to file
	err = os.WriteFile("config.json", configJson, 0644)
	if err != nil {
		fmt.Printf("Creating config: %v", err)
		return
	}
}

func LoadPairs() ([][]string, error) {
	pairsJson, err := os.ReadFile(PAIRS_JSON)
	if err != nil {
		return nil, err
	}

	var pairs [][]string
	err = json.Unmarshal(pairsJson, &pairs)
	if err != nil {
		return nil, err
	}

	return pairs, nil
}

func LoadTickers() ([]string, error) {
	tickersList, err := os.ReadFile(TICKERS_TXT)
	if err != nil {
		return nil, err
	}

	tickers := strings.Split(string(tickersList), ",")

	return tickers, nil
}

func NewConfig(cycle string, path []string) (*crypto.Config, error) {
	currencies := strings.Split(cycle, "-->")
	config := &crypto.Config{
		Pairs: path,
		Types: make(map[string]crypto.OrderType),
	}

	// code:
	// if ticker start with eth => ethusd
	// 		sell eth
	// else ticker starts with usd => usdeth
	// 		buy usd
	// In both scenarios we are reducing eth amount and increase usd amount

	for i := 0; i < len(currencies); i++ {
		c := currencies[i]
		pair := path[i]

		if !strings.Contains(pair, c) {
			return nil, fmt.Errorf("Invalid path or cycle")
		}

		if strings.Index(pair, c) == 0 {
			config.Types[pair] = crypto.SELL
		} else {
			config.Types[pair] = crypto.BUY
		}
	}

	return config, nil
}

func Loading(graph *Graph, cycleLength int, findCyclesF func(*Graph, int) map[string]string) map[string]string {
	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	s.Suffix = " Loading ..."

	s.Start()
	cycles := findCyclesF(graph, cycleLength)
	s.Stop()

	return cycles
}

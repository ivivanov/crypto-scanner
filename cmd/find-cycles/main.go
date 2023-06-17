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

const (
	PAIRS_JSON  = "pairs.json"
	TICKERS_TXT = "tickers.txt" // comma delimited list of tickers
	CYCLE_LEN   = 3
)

type App struct {
	Tickers []string
	Pairs   [][]string
}

func newApp() (*App, error) {
	pairs, err := loadPairs()
	if err != nil {
		return nil, err
	}

	tickers, err := loadTickers()
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
	currencies := strings.Split(cycle, "-")
	// convert to: eth-usd-btc-eth
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
	app, err := newApp()
	if err != nil {
		fmt.Printf("Creating app: %v", err)
		return
	}

	graph, err := NewUndirectedGraph(app.Pairs)
	if err != nil {
		fmt.Printf("Creating graph: %v", err)
		return
	}

	cycles := loading(graph, CYCLE_LEN, findCycles)
	res := make(map[string]crypto.Config)
	pairToCycles := make(map[string][]string)

	// build config
	for c := range cycles {
		_, path, err := app.RestoreTickers(c)
		if err != nil {
			fmt.Printf("Restore tickers: %v", err)
			return
		}

		for _, p := range path {
			_, ok := pairToCycles[p]
			if !ok {
				pairToCycles[p] = append(pairToCycles[p], c)
			} else {
				if !arrContains(pairToCycles[p], c) {
					pairToCycles[p] = append(pairToCycles[p], c)
				}
			}
		}

		conf, err := newConfig(c, path)
		if err != nil {
			fmt.Printf("Creating config: %v", err)
			return
		}

		res[c] = *conf
	}

	// convert to json
	pairToCyclesJson, err := json.MarshalIndent(pairToCycles, "", "  ")
	if err != nil {
		fmt.Printf("Creating config: %v", err)
		return
	}
	// write json to file
	err = os.WriteFile("pair-cycles.json", pairToCyclesJson, 0644)
	if err != nil {
		fmt.Printf("Creating config: %v", err)
		return
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

func loadPairs() ([][]string, error) {
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

func loadTickers() ([]string, error) {
	tickersList, err := os.ReadFile(TICKERS_TXT)
	if err != nil {
		return nil, err
	}

	tickers := strings.Split(string(tickersList), ",")

	return tickers, nil
}

func newConfig(cycle string, path []string) (*crypto.Config, error) {
	currencies := strings.Split(cycle, "-")
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

func loading(graph *Graph, cycleLength int, findCyclesF func(*Graph, int) map[string]string) map[string]string {
	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	s.Suffix = " Loading ..."

	s.Start()
	cycles := findCyclesF(graph, cycleLength)
	s.Stop()

	return cycles
}

func arrContains(arr []string, item string) bool {
	for _, v := range arr {
		if item == v {
			return true
		}
	}

	return false
}

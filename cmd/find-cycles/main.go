package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/briandowns/spinner"
)

type CurrencyPair struct {
	From string
	To   string
}

const (
	PAIRS_JSON = "pairs.json"
)

func main() {
	pairs, err := LoadPairs()
	if err != nil {
		fmt.Printf("load pairs: %v", err)
		return
	}

	var g Graph

	for _, p := range pairs {
		_ = g.AddVertex(p[0])
		_ = g.AddVertex(p[1])

		err := g.AddEdge(p[0], p[1])
		if err != nil {
			fmt.Println(err)
		}
	}

	g.Print()

	cycleLength := 3

	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	s.Suffix = " Loading ..."
	s.Start()

	cycles := findCycles(&g, cycleLength)
	s.Stop()

	fmt.Printf("Cycles of length %d:\n", cycleLength)

	// get only cycles starting with eur
	for key := range cycles {
		fmt.Println(key)
	}

	//eth --> usd -->btc
	//sell--> buy --> sell
	//ethusd usdbtc btceth
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

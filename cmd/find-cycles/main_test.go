package main

import (
	"fmt"
	"testing"

	crypto "github.com/ivivanov/crypto-scanner/types"
)

func TestRestoreTickers(t *testing.T) {
	app, err := NewApp()
	if err != nil {
		fmt.Printf("Creating app: %v", err)
		return
	}

	input := "eth-->usd-->btc"
	expStart := "eth"
	expOut := []string{"ethusd", "btcusd", "ethbtc"}

	actStart, actOut, err := app.RestoreTickers(input)
	if err != nil {
		t.Fatal("Unexpected error")
	}

	if expStart != actStart {
		t.Fatal("Not Matching")
	}

	if len(expOut) != len(actOut) {
		t.Fatal("Not matching")
	}

	for i := 0; i < len(actOut); i++ {
		if expOut[i] != actOut[i] {
			t.Fatal("Not matching")
		}
	}
}

func TestNewConfig(t *testing.T) {
	cycle := "eth-->usd-->btc"
	path := []string{"ethusd", "btcusd", "ethbtc"}

	// "example": {
	// 	"pairs": ["eurusd", "usdtusd", "usdteur"],
	// 	"types": {
	// 	  "eurusd": "sell",
	// 	  "usdtusd": "buy",
	// 	  "usdteur": "sell"
	// 	}
	//   }
	expOut := crypto.Config{
		Pairs: path,
		Types: map[string]crypto.OrderType{
			"ethusd": crypto.SELL,
			"btcusd": crypto.BUY,
			"ethbtc": crypto.BUY,
		},
	}

	actOut, err := NewConfig(cycle, path)
	if err != nil {
		t.Fatal("Unexpected error")
	}

	if len(expOut.Pairs) != len(actOut.Pairs) {
		t.Fatal("Not matching")
	}

	for i := 0; i < len(actOut.Pairs); i++ {
		expPair := expOut.Pairs[i]
		if expPair != actOut.Pairs[i] {
			t.Fatal("Not matching")
		}

		if expOut.Types[expPair] != actOut.Types[expPair] {
			t.Fatal("Not matching")
		}
	}
}

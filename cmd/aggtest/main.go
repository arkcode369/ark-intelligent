package main

import (
	"fmt"
	"time"
	
	"github.com/arkcode369/ark-intelligent/internal/domain"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
)

func main() {
	// Generate 100 fake 15m bars
	bars := make([]domain.IntradayBar, 100)
	base := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	for i := range bars {
		bars[i] = domain.IntradayBar{
			ContractCode: "099741",
			Symbol:       "EUR/USD",
			Interval:     "15m",
			Timestamp:    base.Add(time.Duration(i) * 15 * time.Minute),
			Open:         1.08 + float64(i)*0.0001,
			High:         1.08 + float64(i)*0.0001 + 0.0005,
			Low:          1.08 + float64(i)*0.0001 - 0.0005,
			Close:        1.08 + float64(i)*0.0001 + 0.0002,
			Volume:       1000,
			Source:       "test",
		}
	}
	
	result := pricesvc.AggregateFromBase(bars)
	for _, iv := range []string{"15m", "30m", "1h", "4h", "6h", "12h"} {
		b := result[iv]
		fmt.Printf("%s: %d bars", iv, len(b))
		if len(b) > 0 {
			fmt.Printf(" (Interval=%q)", b[0].Interval)
		}
		fmt.Println()
	}
}

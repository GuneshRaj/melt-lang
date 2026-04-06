package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	outPath := filepath.Join("data", "prices_100k.csv")
	if err := os.MkdirAll("data", 0o755); err != nil {
		panic(err)
	}
	f, err := os.Create(outPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write([]string{"ts", "close"}); err != nil {
		panic(err)
	}
	for i := 1; i <= 100000; i++ {
		close := 100.0 + float64(i%1000)*0.01
		if err := w.Write([]string{fmt.Sprintf("%d", i), fmt.Sprintf("%.2f", close)}); err != nil {
			panic(err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		panic(err)
	}
	fmt.Println(outPath)
}

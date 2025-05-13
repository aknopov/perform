package main

import (
	"flag"
	"os"

	"github.com/aknopov/fancylogger"
)

type SumRequest struct {
	Length int `json:"length"`
}

type SumResponse struct {
	Sum string `json:"sum"`
}

const (
	Port = 8080
)

var (
	logger = fancylogger.NewLogger(os.Stderr, fancylogger.LiteFg)
)

func main() {
	minDelay := flag.Int("min", 0, "minimum response delay (msec)")
	maxDelay := flag.Int("max", 0, "maximum response delay (msec)")
	flag.Parse()
	logger.Info().Msgf("Using minDelay=%d, maxDelay=%d", *minDelay, *maxDelay)

	startGin(Port, *minDelay, *maxDelay)
}

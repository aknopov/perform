package main

import "flag"

type HashRequest struct {
	Password string `json:"password"`
	Strength int    `json:"strength"`
}

type HashResponse struct {
	Hash string `json:"hash"`
}

const (
	Port = 8080
)

func main() {
	minDelay := flag.Int("min", 0, "minimum response delay (msec)")
	maxDelay := flag.Int("max", 0, "maximum response delay (msec)")

	startGin(Port, *minDelay, *maxDelay)
}

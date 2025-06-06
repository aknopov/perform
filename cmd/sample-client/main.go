package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"

	"os"
	"time"

	"github.com/aknopov/fancylogger"
	"github.com/aknopov/perform"
)

const (
	Port      = 8080
	Host      = "localhost"
	WaitSleep = 1 * time.Second
)

type SumRequest struct {
	Length int `json:"length"`
}

type SumResponse struct {
	Sum string `json:"sum"`
}

var (
	logger  = fancylogger.NewLogger(os.Stdout, fancylogger.LiteFg)
	quitReq = perform.AssertNoErr(json.Marshal(SumRequest{-1}))
)

func main() {

	length := flag.Int("l", 10000, "series length")
	concur := flag.Int("c", 10, "concurrent tasks")
	totalTests := flag.Int("n", 500, "total tasks")
	printRaw := flag.Bool("r", false, "print raw durations")
	flag.Parse()

	requestUrl := fmt.Sprintf("http://%s:%d", Host, Port)
	pwdReq := SumRequest{*length}
	jsonString := perform.AssertNoErr(json.Marshal(pwdReq))
	task := func() error { return sendOneRequest(requestUrl, jsonString) }

	waitServer(requestUrl, 5*time.Minute)

	startTime := time.Now()
	stats := perform.RunTest([]perform.TestTask{task}, *totalTests, *concur)
	elapsedTime := time.Since(startTime)

	//nolint:errcheck
	sendOneRequest(requestUrl, []byte(quitReq))

	logger.Info().Msgf("Test finished for the length %d:", pwdReq.Length)
	logger.Info().Int("  num tests", stats[0].Count).Send()
	logger.Info().Int("  num concur", *concur).Send()
	logger.Info().Int("  num failures", stats[0].Fails).Send()
	logger.Info().Dur("  duration (ms)", elapsedTime).Send()
	logger.Info().Float64("  max (ms)", stats[0].MaxTime).Send()
	logger.Info().Float64("  med (ms)", stats[0].MedTime).Send()
	logger.Info().Float64("  min (ms)", stats[0].MinTime).Send()
	logger.Info().Float64("  avg (ms)", stats[0].AvgTime).Send()
	logger.Info().Float64("  stdev (ms)", stats[0].StdDev).Send()

	if *printRaw {
		fmt.Printf("        Raw test durations (ms):\n")
		for i := range *totalTests {
			fmt.Printf("%4d\t%.4f\n", i+1, stats[0].Values[i])
		}
	}
}

func sendOneRequest(url string, jsonString []byte) error {
	bodyReader := bytes.NewReader(jsonString)
	req, err := http.NewRequest(http.MethodPost, url, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	hashRaw, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	var hashResp SumResponse
	err = json.Unmarshal(hashRaw, &hashResp)
	if err != nil {
		return err
	}

	// logger.Debug().Msgf("Response: %s (%d)", hashResp.Hash, res.StatusCode)
	return nil
}

func waitServer(url string, timeout time.Duration) {
	logger.Debug().Msg("Waiting for server ...")
	count := timeout / WaitSleep
	req, _ := http.NewRequest(http.MethodHead, url, nil)

	for range count {
		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp != nil {
			logger.Debug().Msg("Endpoint is open now")
			return
		}
		time.Sleep(WaitSleep)
	}
	logger.Error().Str("url", url).Dur("wait", timeout).Msg("Connectioon timed-out")
	os.Exit(1)
}

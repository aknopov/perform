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
	Port            = 8080
	Host            = "localhost"
	TotalTests      = 500
	WaitSleep       = 1 * time.Second
)

type HashRequest struct {
	Password string `json:"password"`
	Strength int    `json:"strength"`
}

type HashResponse struct {
	Hash string `json:"hash"`
}

var (
	logger  = fancylogger.NewLogger(os.Stdout, fancylogger.LiteFg)
	quitReq = assertNoErr(json.Marshal(HashRequest{"quit", 0}))
)

func main() {

	strength := flag.Int("s", 8, "encryption passes")
	concur := flag.Int("c", 10, "concurrent requests")
	flag.Parse()

	requestUrl := fmt.Sprintf("http://%s:%d", Host, Port)
	pwdReq := HashRequest{"A secret", *strength}
	jsonString := assertNoErr(json.Marshal(pwdReq))
	task := func() error { return sendOneRequest(requestUrl, jsonString) }

	waitServer(requestUrl, 5*time.Minute)

	startTime := time.Now()
	stats := perform.RunTest([]perform.TestTask{task}, TotalTests, *concur)
	elapsedTime := time.Since(startTime)

	assertNoErr(struct{}{}, sendOneRequest(requestUrl, []byte(quitReq)))

	logger.Info().Msgf("Test finished for the factor %d:", pwdReq.Strength)
	logger.Info().Int("  num tests", stats[0].Count).Send()
	logger.Info().Int("  num concur", *concur).Send()
	logger.Info().Int("  num failures", stats[0].Fails).Send()
	logger.Info().Dur("  duration (ms)", elapsedTime).Send()
	logger.Info().Dur("  sum (ms)", stats[0].TotalTime).Send()
	logger.Info().Dur("  max (ms)", stats[0].MaxTime).Send()
	logger.Info().Dur("  med (ms)", stats[0].MedTime).Send()
	logger.Info().Dur("  min (ms)", stats[0].MinTime).Send()
	logger.Info().Dur("  avg (ms)", stats[0].AvgTime).Send()
	logger.Info().Dur("  stdev (ms)", stats[0].StdDev).Send()
}

func assertNoErr[T any](val T, err error) T {
	if err != nil {
		logger.Fatal().Err(err)
	}
	return val
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

	var hashResp HashResponse
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

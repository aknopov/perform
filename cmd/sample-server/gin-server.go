package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/aknopov/perform"
	"github.com/gin-gonic/gin"
)

var (
	stopChan = make(chan struct{}, 1)
)

func startGin(port, minDelay, maxDelay int) {
	gin.SetMode(gin.ReleaseMode)

	engine := gin.New()
	engine.Use(gin.Recovery()) // no debug logging
	perform.AssertNoErr(perform.ND, engine.SetTrustedProxies(nil))
	engine.POST("/", func(ctx *gin.Context) { calcSum4Gin(ctx, minDelay, maxDelay) })

	logger.Info().Msg("-- Starting server...")
	go func() { perform.AssertNoErr(perform.ND, engine.Run(fmt.Sprintf(":%d", port))) }()

	<-stopChan
	os.Exit(0)
}

func calcSum4Gin(ctx *gin.Context, minDelay, maxDelay int) {
	request := new(SumRequest)

	perform.AssertNoErr(perform.ND, ctx.BindJSON(&request))

	if request.Length == -1 {
		logger.Info().Msg("-- Stopping server...")
		ctx.JSON(http.StatusOK, SumResponse{"done"})
		stopChan <- perform.ND
		return
	}

	sumVal := calcSum(request.Length).String()

	// Sleep random number of milliseconds if required
	if maxDelay > 0 {
		msecs := minDelay
		if maxDelay > minDelay {
			msecs = rand.Intn(maxDelay - minDelay)
		}
		time.Sleep(time.Duration(msecs) * time.Millisecond)
	}

	ctx.JSON(http.StatusOK, SumResponse{sumVal})
}

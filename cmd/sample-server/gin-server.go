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
	engine.POST("/", func(ctx *gin.Context) { hashPassword4Gin(ctx, minDelay, maxDelay) })

	logger.Info().Msg("-- Starting server...")
	go func() { perform.AssertNoErr(perform.ND, engine.Run(fmt.Sprintf(":%d", port))) }()

	<-stopChan
	os.Exit(0)
}

func hashPassword4Gin(ctx *gin.Context, minDelay, maxDelay int) {
	request := new(HashRequest)

	perform.AssertNoErr(perform.ND, ctx.BindJSON(&request))

	if request.Password == "quit" {
		logger.Info().Msg("-- Stopping server...")
		ctx.JSON(http.StatusOK, HashResponse{"done"})
		stopChan <- perform.ND
		return
	}

	hashCode := hashStr(request)

	// Sleep random number of milliseconds if required
	if maxDelay > 0 {
		msecs := minDelay
		if maxDelay > minDelay {
			msecs = rand.Intn(maxDelay - minDelay)
		}
		time.Sleep(time.Duration(msecs) * time.Millisecond)
	}

	ctx.JSON(http.StatusOK, HashResponse{hashCode})
}

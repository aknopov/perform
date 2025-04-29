package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/aknopov/fancylogger"
	"github.com/aknopov/perform"
	"github.com/gin-gonic/gin"
)

var (
	logger   = fancylogger.NewLogger(os.Stderr, fancylogger.LiteFg)
	stopChan = make(chan struct{}, 1)
	// minSleep = 1000
	// maxSleep = 10000
)

func startGin(port int) {
	gin.SetMode(gin.ReleaseMode)

	engine := gin.New()
	engine.Use(gin.Recovery()) // no debug logging
	perform.AssertNoErr(perform.ND, engine.SetTrustedProxies(nil))
	engine.POST("/", hashPassword4Gin)

	logger.Info().Msg("-- Starting server...")
	go func() { perform.AssertNoErr(perform.ND, engine.Run(fmt.Sprintf(":%d", port))) }()

	<-stopChan
	os.Exit(0)
}

func hashPassword4Gin(ctx *gin.Context) {
	request := new(HashRequest)

	perform.AssertNoErr(perform.ND, ctx.BindJSON(&request))

	if request.Password == "quit" {
		logger.Info().Msg("-- Stopping server...")
		ctx.JSON(http.StatusOK, HashResponse{"done"})
		stopChan <- perform.ND
		return
	}

	// logger.Debug().
	// 	Str("text", request.Password).
	// 	Msg("Requested hash for")

	hashCode := hashStr(request)

	// Sleep 1-10 secs depending on the time since app start
	// time.Sleep(time.Duration(minSleep+rand.Intn(maxSleep-minSleep)) * time.Millisecond)

	ctx.JSON(http.StatusOK, HashResponse{hashCode})
}

package main

import (
	"log"
	"os"
	"os/signal"

	"sydney/provider"
	"sydney/tg"

	"go.uber.org/zap"
	"gopkg.in/natefinch/lumberjack.v2"
)

func main() {
	config, err := provider.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	loggerCfg := config.LoggerConfig
	jackLogger := &lumberjack.Logger{
		Filename:   loggerCfg.Filename,
		MaxSize:    loggerCfg.MaxSize,
		MaxAge:     loggerCfg.MaxAge,
		MaxBackups: loggerCfg.MaxBackups,
		LocalTime:  loggerCfg.LocalTime,
		Compress:   loggerCfg.Compress,
	}

	logger := provider.NewLogger(loggerCfg.Level, "console", jackLogger)

	bot := tg.NewBot(logger, config.TelegramBotConfig)
	if err := bot.Start(); err != nil {
		logger.Error("bot start", zap.String("err", err.Error()))
		return
	}

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit

	if err := bot.Stop(); err != nil {
		logger.Error("bot stop", zap.String("err", err.Error()))
		return
	}
}

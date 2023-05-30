package logger

import (
    "go.uber.org/zap"
    "log"
)

func InitLogger() *zap.SugaredLogger {
    logger, err := zap.NewDevelopment()

    // disable DEBUG level in prod
    // logger, err := zap.NewProduction()
    if err != nil {
        log.Fatalf("can't initialize zap logger: %v", err)
    }
    defer logger.Sync() // flushes buffer, if any
    return logger.Sugar()
}
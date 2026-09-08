package lib

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

func Init(level zerolog.Level) {
	fileLogger := lumberjack.Logger{
		Filename:   "/var/log/mimi/mimi.log",
		MaxSize:    10,
		MaxAge:     20,
		MaxBackups: 3,
		Compress:   false,
	}

	if level == zerolog.DebugLevel {
		zerolog.TimeFieldFormat = time.RFC3339
	} else {
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	}

	zerolog.SetGlobalLevel(level)

	log.Logger = zerolog.New(io.MultiWriter(os.Stdout, &fileLogger)).
		With().
		Timestamp().
		Caller().
		Logger()
}

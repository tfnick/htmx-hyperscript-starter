package logging

import (
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/rs/zerolog"
)

const DefaultLogPath = "logs/app.log"

var (
	baseWriter io.Writer = os.Stdout
	logFile    *os.File
	isDevMode  bool
	level      = zerolog.InfoLevel
	mu         sync.RWMutex
)

type sinkWriter struct{}

func (sinkWriter) Write(p []byte) (int, error) {
	mu.RLock()
	defer mu.RUnlock()

	return baseWriter.Write(p)
}

func Init(isDevelopment bool) error {
	mu.Lock()
	defer mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(DefaultLogPath), 0755); err != nil {
		return err
	}

	file, err := os.OpenFile(DefaultLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	if logFile != nil {
		_ = logFile.Close()
	}

	isDevMode = isDevelopment
	level = zerolog.InfoLevel
	if isDevelopment {
		level = zerolog.DebugLevel
	}

	logFile = file
	baseWriter = io.MultiWriter(os.Stdout, file)
	return nil
}

func For(component string) zerolog.Logger {
	mu.RLock()
	defer mu.RUnlock()

	return zerolog.New(sinkWriter{}).
		Level(level).
		With().
		Timestamp().
		Str("component", component).
		Logger()
}

func IsDevelopment() bool {
	mu.RLock()
	defer mu.RUnlock()

	return isDevMode
}

func Close() error {
	mu.Lock()
	defer mu.Unlock()

	if logFile == nil {
		return nil
	}

	err := logFile.Close()
	logFile = nil
	baseWriter = os.Stdout
	return err
}

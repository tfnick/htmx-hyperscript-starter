package logging

import (
	"os"
	"strings"
	"testing"
)

func TestInitWritesToLogFile(t *testing.T) {
	t.Cleanup(func() {
		if err := Close(); err != nil {
			t.Fatalf("close logging: %v", err)
		}
		if err := os.RemoveAll("logs"); err != nil {
			t.Fatalf("remove logs: %v", err)
		}
	})

	if err := Init(true); err != nil {
		t.Fatalf("init logging: %v", err)
	}

	logger := For("test")
	logger.Info().Str("surface", "app").Msg("file logging works")

	if err := Close(); err != nil {
		t.Fatalf("close logging: %v", err)
	}

	content, err := os.ReadFile(DefaultLogPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}

	logs := string(content)
	if !strings.Contains(logs, `"component":"test"`) {
		t.Fatalf("expected component field in log file, got %s", logs)
	}
	if !strings.Contains(logs, `"surface":"app"`) {
		t.Fatalf("expected surface field in log file, got %s", logs)
	}
	if !strings.Contains(logs, `"message":"file logging works"`) {
		t.Fatalf("expected message in log file, got %s", logs)
	}
}

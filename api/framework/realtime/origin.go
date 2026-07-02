package realtime

import (
	"os"
	"strings"
)

const EnvWebSocketOriginPatterns = "APP_WEBSOCKET_ORIGIN_PATTERNS"

var defaultWebSocketOriginPatterns = []string{"127.0.0.1:5173", "localhost:5173"}

func WebSocketOriginPatternsFromEnv() []string {
	configured := strings.TrimSpace(os.Getenv(EnvWebSocketOriginPatterns))
	if configured == "" {
		return append([]string(nil), defaultWebSocketOriginPatterns...)
	}

	parts := strings.FieldsFunc(configured, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	})
	patterns := make([]string, 0, len(parts))
	seen := make(map[string]bool)
	for _, part := range parts {
		pattern := strings.TrimSpace(part)
		if pattern == "" || seen[pattern] {
			continue
		}
		seen[pattern] = true
		patterns = append(patterns, pattern)
	}
	if len(patterns) == 0 {
		return append([]string(nil), defaultWebSocketOriginPatterns...)
	}
	return patterns
}

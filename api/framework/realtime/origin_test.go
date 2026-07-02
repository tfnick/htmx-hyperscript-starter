package realtime

import "testing"

func TestWebSocketOriginPatternsFromEnvUsesDefaultLocalhostPatterns(t *testing.T) {
	t.Setenv(EnvWebSocketOriginPatterns, "")

	patterns := WebSocketOriginPatternsFromEnv()
	if len(patterns) != 2 || patterns[0] != "127.0.0.1:5173" || patterns[1] != "localhost:5173" {
		t.Fatalf("unexpected default websocket origin patterns: %#v", patterns)
	}
}

func TestWebSocketOriginPatternsFromEnvParsesConfiguredAllowlist(t *testing.T) {
	t.Setenv(EnvWebSocketOriginPatterns, "app.example.com, admin.example.com; app.example.com\n127.0.0.1:5173")

	patterns := WebSocketOriginPatternsFromEnv()
	expected := []string{"app.example.com", "admin.example.com", "127.0.0.1:5173"}
	if len(patterns) != len(expected) {
		t.Fatalf("unexpected pattern count: %#v", patterns)
	}
	for i := range expected {
		if patterns[i] != expected[i] {
			t.Fatalf("unexpected pattern at %d: got %q want %q in %#v", i, patterns[i], expected[i], patterns)
		}
	}
}

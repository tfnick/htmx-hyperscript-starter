package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// EnvFileLoadResult reports a processed env file without exposing values.
type EnvFileLoadResult struct {
	Path     string
	Assigned int
}

// LoadEnvFiles reads dotenv-style files and sets only variables that are not
// already present in the process environment.
func LoadEnvFiles(paths ...string) ([]EnvFileLoadResult, error) {
	seen := make(map[string]bool)
	results := make([]EnvFileLoadResult, 0, len(paths))

	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}

		cleanPath, err := filepath.Abs(path)
		if err != nil {
			cleanPath = filepath.Clean(path)
		}
		if seen[cleanPath] {
			continue
		}
		seen[cleanPath] = true

		file, err := os.Open(cleanPath)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return results, fmt.Errorf("open env file %s: %w", cleanPath, err)
		}

		assigned, loadErr := loadEnv(file)
		closeErr := file.Close()
		if loadErr != nil {
			return results, fmt.Errorf("load env file %s: %w", cleanPath, loadErr)
		}
		if closeErr != nil {
			return results, fmt.Errorf("close env file %s: %w", cleanPath, closeErr)
		}

		results = append(results, EnvFileLoadResult{Path: cleanPath, Assigned: assigned})
	}

	return results, nil
}

func loadEnv(reader io.Reader) (int, error) {
	scanner := bufio.NewScanner(reader)
	assigned := 0
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		rawLine := scanner.Text()
		if lineNumber == 1 {
			rawLine = strings.TrimPrefix(rawLine, "\ufeff")
		}
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		key, rawValue, ok := strings.Cut(line, "=")
		if !ok {
			return assigned, fmt.Errorf("line %d: expected KEY=value", lineNumber)
		}

		key = strings.TrimSpace(key)
		if !isValidEnvKey(key) {
			return assigned, fmt.Errorf("line %d: invalid env key %q", lineNumber, key)
		}

		value, err := parseEnvValue(rawValue)
		if err != nil {
			return assigned, fmt.Errorf("line %d: %w", lineNumber, err)
		}

		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return assigned, fmt.Errorf("line %d: set env key %q: %w", lineNumber, key, err)
		}
		assigned++
	}
	if err := scanner.Err(); err != nil {
		return assigned, err
	}

	return assigned, nil
}

func parseEnvValue(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}

	switch value[0] {
	case '"':
		return parseDoubleQuotedEnvValue(value)
	case '\'':
		return parseSingleQuotedEnvValue(value)
	}

	return trimEnvInlineComment(value), nil
}

func parseDoubleQuotedEnvValue(value string) (string, error) {
	escaped := false
	var parsed strings.Builder
	for i := 1; i < len(value); i++ {
		char := value[i]
		if escaped {
			switch char {
			case 'n':
				parsed.WriteByte('\n')
			case 'r':
				parsed.WriteByte('\r')
			case 't':
				parsed.WriteByte('\t')
			case '"', '\\':
				parsed.WriteByte(char)
			default:
				parsed.WriteByte('\\')
				parsed.WriteByte(char)
			}
			escaped = false
			continue
		}

		switch char {
		case '\\':
			escaped = true
		case '"':
			tail := strings.TrimSpace(value[i+1:])
			if tail != "" && !strings.HasPrefix(tail, "#") {
				return "", fmt.Errorf("unexpected content after quoted value")
			}
			return parsed.String(), nil
		default:
			parsed.WriteByte(char)
		}
	}
	return "", fmt.Errorf("invalid quoted value")
}

func parseSingleQuotedEnvValue(value string) (string, error) {
	end := strings.Index(value[1:], "'")
	if end < 0 {
		return "", fmt.Errorf("invalid quoted value")
	}
	end++
	tail := strings.TrimSpace(value[end+1:])
	if tail != "" && !strings.HasPrefix(tail, "#") {
		return "", fmt.Errorf("unexpected content after quoted value")
	}
	return value[1:end], nil
}

func trimEnvInlineComment(value string) string {
	for i, r := range value {
		if r == '#' && (i == 0 || unicode.IsSpace(rune(value[i-1]))) {
			return strings.TrimSpace(value[:i])
		}
	}
	return strings.TrimSpace(value)
}

func isValidEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		if i == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

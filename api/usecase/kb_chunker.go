package usecase

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/google/uuid"
)

// Chunk represents a text chunk produced by the chunker.
type KBChunk struct {
	ID          string
	Content     string
	ContentHash string
	TokenCount  int
	CharCount   int
}

// SimpleChunker splits text into chunks of approximately maxTokens each.
// It splits by paragraphs first, then merges paragraph groups up to the token limit.
// Token count is estimated using a rough heuristic: words * 1.3.
type SimpleChunker struct {
	// MaxTokens is the approximate maximum token count per chunk.
	// Defaults to 500 if not set.
	MaxTokens int
}

// Chunk splits the given text into KBChunk slices.
func (c *SimpleChunker) Chunk(text string) []KBChunk {
	maxTokens := c.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 500
	}

	if strings.TrimSpace(text) == "" {
		return nil
	}

	// Split by paragraphs (double newline)
	paragraphs := splitParagraphs(text)

	// Merge paragraphs into chunks
	var chunks []KBChunk
	var currentParagraphs []string
	currentTokens := 0
	currentChars := 0

	for _, para := range paragraphs {
		paraTokens := estimateTokens(para)
		paraChars := len([]rune(para))

		// If a single paragraph exceeds max tokens, split it further
		if paraTokens > maxTokens {
			// First flush any accumulated paragraphs
			if len(currentParagraphs) > 0 {
				chunks = append(chunks, buildChunk(currentParagraphs, currentTokens, currentChars))
				currentParagraphs = nil
				currentTokens = 0
				currentChars = 0
			}
			// Split the long paragraph into sentence-sized pieces
			subChunks := splitLongParagraph(para, maxTokens)
			chunks = append(chunks, subChunks...)
			continue
		}

		// Check if adding this paragraph would exceed the limit
		if len(currentParagraphs) > 0 && currentTokens+paraTokens > maxTokens {
			chunks = append(chunks, buildChunk(currentParagraphs, currentTokens, currentChars))
			currentParagraphs = nil
			currentTokens = 0
			currentChars = 0
		}

		currentParagraphs = append(currentParagraphs, para)
		currentTokens += paraTokens
		currentChars += paraChars
	}

	// Flush remaining paragraphs
	if len(currentParagraphs) > 0 {
		chunks = append(chunks, buildChunk(currentParagraphs, currentTokens, currentChars))
	}

	return chunks
}

// splitParagraphs splits text by double newlines and trims each paragraph.
func splitParagraphs(text string) []string {
	raw := strings.Split(text, "\n\n")
	var result []string
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		result = append(result, p)
	}
	return result
}

// splitLongParagraph splits a paragraph that exceeds maxTokens into smaller chunks
// by breaking at sentence boundaries (., !, ?) followed by a space.
func splitLongParagraph(para string, maxTokens int) []KBChunk {
	sentences := splitSentences(para)
	var chunks []KBChunk
	var current []string
	currentTokens := 0
	currentChars := 0

	for _, sentence := range sentences {
		sentenceTokens := estimateTokens(sentence)
		sentenceChars := len([]rune(sentence))

		if len(current) > 0 && currentTokens+sentenceTokens > maxTokens {
			chunks = append(chunks, buildChunk(current, currentTokens, currentChars))
			current = nil
			currentTokens = 0
			currentChars = 0
		}

		current = append(current, sentence)
		currentTokens += sentenceTokens
		currentChars += sentenceChars
	}

	if len(current) > 0 {
		chunks = append(chunks, buildChunk(current, currentTokens, currentChars))
	}

	return chunks
}

// splitSentences splits text on sentence boundaries.
func splitSentences(text string) []string {
	var sentences []string
	runes := []rune(text)
	start := 0

	for i := 0; i < len(runes); i++ {
		if runes[i] == '.' || runes[i] == '!' || runes[i] == '?' {
			// Look ahead for a space, newline, or end of text
			if i+1 >= len(runes) || runes[i+1] == ' ' || runes[i+1] == '\n' {
				sentence := strings.TrimSpace(string(runes[start : i+1]))
				if sentence != "" {
					sentences = append(sentences, sentence)
				}
				start = i + 1
				if i+1 < len(runes) && runes[i+1] == ' ' {
					start = i + 2
				}
			}
		}
	}

	// Capture remaining text
	if start < len(runes) {
		remaining := strings.TrimSpace(string(runes[start:]))
		if remaining != "" {
			sentences = append(sentences, remaining)
		}
	}

	return sentences
}

// estimateTokens provides a rough estimate of token count using word count * 1.3.
func estimateTokens(text string) int {
	words := strings.Fields(text)
	// Rough estimate: words * 1.3, rounded up
	tokens := int(float64(len(words)) * 1.3)
	if tokens < 1 && len(strings.TrimSpace(text)) > 0 {
		return 1
	}
	return tokens
}

// buildChunk creates a KBChunk from accumulated paragraphs.
func buildChunk(paragraphs []string, tokenCount, charCount int) KBChunk {
	content := strings.Join(paragraphs, "\n\n")
	hash := sha256.Sum256([]byte(content))
	return KBChunk{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Content:     content,
		ContentHash: hex.EncodeToString(hash[:]),
		TokenCount:  tokenCount,
		CharCount:   charCount,
	}
}

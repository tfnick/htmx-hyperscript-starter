package localhash

import (
	"context"
	"hash/fnv"
	"math"
	"strings"
	"unicode"

	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/embedding"
)

const defaultDimensions = 64

type Adapter struct {
	dimensions int
}

func NewAdapter(dimensions int) *Adapter {
	if dimensions <= 0 {
		dimensions = defaultDimensions
	}
	return &Adapter{dimensions: dimensions}
}

func (a *Adapter) Embed(_ context.Context, cfg embedding.ProviderConfig, req embedding.EmbedRequest) (embedding.EmbedResult, error) {
	dimensions := a.dimensions
	vectors := make([]embedding.Vector, 0, len(req.Texts))
	totalTokens := 0
	for _, text := range req.Texts {
		values, tokenCount := hashEmbedding(text, dimensions)
		totalTokens += tokenCount
		vectors = append(vectors, embedding.Vector{Values: values})
	}

	modelCode := strings.TrimSpace(cfg.ModelCode)
	if modelCode == "" {
		modelCode = "local-hash-64"
	}
	providerModelID := strings.TrimSpace(cfg.ProviderModelID)
	if providerModelID == "" {
		providerModelID = modelCode
	}

	return embedding.EmbedResult{
		Vectors:         vectors,
		ModelCode:       modelCode,
		ProviderModelID: providerModelID,
		Dimensions:      dimensions,
		Usage:           embedding.Usage{PromptTokens: totalTokens, TotalTokens: totalTokens},
	}, nil
}

func hashEmbedding(text string, dimensions int) ([]float32, int) {
	values := make([]float32, dimensions)
	tokens := tokenize(text)
	for _, token := range tokens {
		index := int(hashToken(token) % uint32(dimensions))
		values[index] += 1
	}
	normalize(values)
	return values, len(tokens)
}

func tokenize(text string) []string {
	var tokens []string
	var builder strings.Builder
	flush := func() {
		if builder.Len() == 0 {
			return
		}
		tokens = append(tokens, strings.ToLower(builder.String()))
		builder.Reset()
	}

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return tokens
}

func hashToken(token string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(token))
	return h.Sum32()
}

func normalize(values []float32) {
	var sum float64
	for _, value := range values {
		sum += float64(value * value)
	}
	if sum == 0 {
		return
	}
	scale := 1 / math.Sqrt(sum)
	for i := range values {
		values[i] = float32(float64(values[i]) * scale)
	}
}

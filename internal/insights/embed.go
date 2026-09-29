package insights

import (
	"context"
	"errors"
	"math"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// Dimensions is the width of a passage vector.
const Dimensions = 512

// OpenAIVectors names the vectors OpenAI makes. Every vector stored before the
// embedder could be chosen is one of these.
const OpenAIVectors = "openai/text-embedding-3-small/512"

// localVectors names the vectors of the bundled local model.
const localVectors = "local/qwen3-embedding-0.6b-q8_0/512"

// localQuery is the instruction Qwen3-Embedding expects on the query side;
// passages are embedded as they are.
const localQuery = "Instruct: Given a question, retrieve passages from meeting transcripts and notes that answer it\nQuery: "

// Embed turns passages into vectors.
func (c *Client) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if !c.Searchable() {
		return nil, ErrNoKey
	}
	if len(texts) == 0 {
		return nil, nil
	}
	return c.embed(ctx, texts)
}

// Query turns a question into the vector that finds the passages answering it.
func (c *Client) Query(ctx context.Context, question string) ([]float32, error) {
	if !c.Searchable() {
		return nil, ErrNoKey
	}
	vectors, err := c.Embed(ctx, []string{c.query + question})
	if err != nil {
		return nil, err
	}
	if len(vectors) != 1 || len(vectors[0]) != Dimensions {
		return nil, errors.New("the search model returned no vector for the question")
	}
	return vectors[0], nil
}

func openAIVectors(key string) func(context.Context, []string) ([][]float32, error) {
	api := openai.NewClient(option.WithAPIKey(key))
	return func(ctx context.Context, texts []string) ([][]float32, error) {
		resp, err := api.Embeddings.New(ctx, openai.EmbeddingNewParams{
			Model:      openai.EmbeddingModelTextEmbedding3Small,
			Dimensions: openai.Int(Dimensions),
			Input:      openai.EmbeddingNewParamsInputUnion{OfArrayOfStrings: texts},
		})
		if err != nil {
			return nil, err
		}
		return shorten(resp.Data, len(texts)), nil
	}
}

// shorten lines vectors up with their passages by index and cuts each to
// Dimensions. OpenAI already returns that width; Qwen3-Embedding returns 1024,
// and is trained so that a prefix of its vector is a vector in its own right.
// Renormalising keeps every stored vector unit length.
func shorten(data []openai.Embedding, n int) [][]float32 {
	out := make([][]float32, n)
	for _, item := range data {
		if int(item.Index) >= n || len(item.Embedding) < Dimensions {
			continue
		}
		var norm float64
		for _, x := range item.Embedding[:Dimensions] {
			norm += x * x
		}
		if norm == 0 {
			continue
		}
		norm = math.Sqrt(norm)
		v := make([]float32, Dimensions)
		for i, x := range item.Embedding[:Dimensions] {
			v[i] = float32(x / norm)
		}
		out[item.Index] = v
	}
	return out
}

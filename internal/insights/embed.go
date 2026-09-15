package insights

import (
	"context"

	"github.com/openai/openai-go/v3"
)

// Dimensions is the width of a passage vector.
const Dimensions = 512

// Embedder is the embeddings model.
const Embedder = openai.EmbeddingModelTextEmbedding3Small

// Embed turns passages into vectors.
func (c *Client) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if !c.Ready() {
		return nil, ErrNoKey
	}
	if len(texts) == 0 {
		return nil, nil
	}
	resp, err := c.api.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model:      Embedder,
		Dimensions: openai.Int(Dimensions),
		Input:      openai.EmbeddingNewParamsInputUnion{OfArrayOfStrings: texts},
	})
	if err != nil {
		return nil, err
	}
	// Returned in order, but indexed anyway: a caller lining vectors up against
	// its own passages by position would be silently wrong if that ever changed.
	out := make([][]float32, len(texts))
	for _, item := range resp.Data {
		if int(item.Index) >= len(out) {
			continue
		}
		v := make([]float32, len(item.Embedding))
		for i, x := range item.Embedding {
			v[i] = float32(x)
		}
		out[item.Index] = v
	}
	return out, nil
}

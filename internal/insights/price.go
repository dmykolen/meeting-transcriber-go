package insights

// prices are OpenAI's list prices in US dollars per million tokens, input then
// output, for the models the app offers by default. They are a snapshot of
// openai.com/api/pricing (2026-10): a model missing here shows tokens only,
// never an invented cost.
var prices = map[string][2]float64{
	"gpt-5.4-mini":           {0.75, 4.50},
	"text-embedding-3-small": {0.02, 0},
}

// Cost is what OpenAI charges for these tokens, in US dollars; false when the
// provider is not billed per token or the model's price is not known.
func Cost(provider, model string, input, output int64) (float64, bool) {
	price, known := prices[model]
	if provider != "openai" || !known {
		return 0, false
	}
	return (float64(input)*price[0] + float64(output)*price[1]) / 1e6, true
}

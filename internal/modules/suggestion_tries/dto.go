package suggestion_tries

type SuggestionTriesResponse struct {
	WeekStart      string  `json:"week_start"`
	TryNumber      int     `json:"try_number"`
	Suggestions    []Movie `json:"suggestions"`
	GeneratedAt    string  `json:"generated_at"`
	RemainingTries int     `json:"remaining_tries"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
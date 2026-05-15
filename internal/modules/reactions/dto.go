package reactions

type AddReactionRequest struct {
	TMDBID   int    `json:"tmdb_id" binding:"required"`
	Reaction string `json:"reaction" binding:"required"`
}

type AddReactionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
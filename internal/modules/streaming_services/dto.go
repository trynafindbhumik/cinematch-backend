package streaming_services

// StreamingServiceResponse is returned when getting a single streaming service
type StreamingServiceResponse struct {
	ID      int16  `json:"id"`
	Name    string `json:"name"`
	IconURL string `json:"iconUrl"`
}

// StreamingServicesListResponse is returned when getting all streaming services
type StreamingServicesListResponse struct {
	StreamingServices []StreamingServiceResponse `json:"streamingServices"`
	NextCursor         string                     `json:"next_cursor,omitempty"`
	Limit              int                        `json:"limit"`
	TotalCount         int                        `json:"total_count"`
}

// UserStreamingServiceResponse is returned when getting user's selected streaming services
type UserStreamingServiceResponse struct {
	ID       int16  `json:"id"`
	Name     string `json:"name"`
	IconURL  string `json:"iconUrl"`
	SourceID int16  `json:"sourceId"` // Foreign key to streaming_services table
}

// UserStreamingServicesListResponse is returned when getting user's selected streaming services
type UserStreamingServicesListResponse struct {
	StreamingServices []UserStreamingServiceResponse `json:"streamingServices"`
}

// UpdateStreamingServicesRequest is used to add streaming services to user's profile
type UpdateStreamingServicesRequest struct {
	ServiceIDs []int16 `json:"serviceIds" binding:"required"`
}

// SearchStreamingServicesRequest is used to search streaming services
type SearchStreamingServicesRequest struct {
	Query string `json:"q" binding:"required"`
}

// SearchStreamingServicesResponse is returned when searching streaming services
type SearchStreamingServicesResponse struct {
	StreamingServices []StreamingServiceResponse `json:"streamingServices"`
	NextCursor         string                     `json:"next_cursor,omitempty"`
	Limit              int                        `json:"limit"`
	TotalCount         int                        `json:"total_count"`
}

// ErrorResponse is the standard error response
type ErrorResponse struct {
	Error string `json:"error"`
}

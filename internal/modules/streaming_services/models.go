package streaming_services

// StreamingService represents a streaming service in the database
type StreamingService struct {
	ID      int16
	Name    string
	IconURL *string
	TMDBID  *int32
}

// UserStreamingService represents a user's selected streaming service
type UserStreamingService struct {
	UserID     int64
	ServiceID  int16
	SourceName string // For convenience when returning to frontend
	IconURL    *string
}

package suggestions

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/trynafindbhumik/cinematch-backend/internal/modules/movies"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/gemini"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/logger"
	"go.uber.org/zap"
)

const minFavorites = 5
const numSuggestions = 20

const systemPrompt = `You are a film recommendation expert with deep knowledge of global cinema — English, Hindi, Tamil, Telugu, Bengali, Korean, Japanese, French, German, Spanish, Chinese, and all other major film industries. You understand film history, directorial styles, acting styles, and what makes a movie memorable.

Provide recommendations based on the user's favorite films, watchlist, and reactions. Focus on matching film styles, themes, directorial approaches, and cinematic elements rather than psychological interpretations.

Provide recommendations ONLY. Do not ask questions. Do not explain your reasoning process. Just output the JSON.`

type Service struct {
	repo         *Repository
	gemini       *gemini.Client
	moviesSvc    *movies.Service
}

func NewService(repo *Repository, geminiClient *gemini.Client, moviesSvc *movies.Service) *Service {
	return &Service{
		repo:      repo,
		gemini:    geminiClient,
		moviesSvc: moviesSvc,
	}
}

type GenerateResult struct {
	Suggestions    []MovieDetails
	GenerationDate string
	Regeneration   bool
	Finished       bool
	Message        string
	IsExhausted    bool
}

type NextResult struct {
	Suggestion   *MovieDetails
	NextTMDBID   *int
	HasMore      bool
	Regeneration bool
	Finished     bool
	Message      string
}

func (s *Service) GenerateSuggestions(ctx context.Context, userID int64) (*GenerateResult, error) {
	log := logger.WithUserID(userID).With(zap.String("operation", "GenerateSuggestions"))

	// Step 1: Check for old log with movie_ids
	oldLog, err := s.repo.FindOldLogWithMovieIDs(ctx, userID)
	if err != nil {
		log.Error("failed to find old log", logger.Err(err))
		return nil, fmt.Errorf("failed to find old log: %w", err)
	}

	if oldLog != nil {
		log.Info("found old log", logger.String("date", oldLog.Date), logger.Int("movie_count", len(oldLog.MovieIDs)))

		movieIDs := oldLog.MovieIDs
		log.Info("checking old log movies one by one", logger.Int("total", len(movieIDs)))
		var validMovies []MovieDetails
		for _, tmdbID := range movieIDs {
			details, err := s.moviesSvc.GetMovieByID(ctx, tmdbID, userID)
			if err != nil {
				log.Warn("skipping stale TMDB ID", logger.Int("tmdb_id", tmdbID))
				continue
			}
			validMovies = append(validMovies, MovieDetails{
				TMDBID:      details.TMDBID,
				Title:       details.Title,
				PosterURL:   details.PosterURL,
				BackdropURL: details.BackdropURL,
				ReleaseYear: details.ReleaseYear,
				TMDBRating:  details.TMDBRating,
				Genres:      details.Genres,
				Runtime:     details.Runtime,
				Tagline:     details.Tagline,
				Status:      details.Status,
				TotalCount:  details.TotalCount,
				LikeCount:   details.LikeCount,
				LoveCount:   details.LoveCount,
				HateCount:   details.HateCount,
				SkipCount:   details.SkipCount,
				DislikeCount: details.DislikeCount,
				UserReaction: details.UserReaction,
			})
			if len(validMovies) == 2 {
				break // return first 2 valid
			}
		}

		if len(validMovies) > 0 {
			regeneration := len(oldLog.MovieIDs) <= 2
			log.Info("returning old log suggestions", logger.Int("suggestions_count", len(validMovies)))
			return &GenerateResult{
				Suggestions:    validMovies,
				GenerationDate: oldLog.Date,
				Regeneration:   regeneration,
				Finished:       false,
			}, nil
		}

		// All old log movies stale — fall through to today's log
		log.Warn("all old log TMDB IDs stale, falling through to today's log")
	}

	// Step 2: Today's log exists with movies — use it
	todayLog, err := s.repo.GetTodayLog(ctx, userID)
	if err != nil {
		log.Error("failed to get today's log", logger.Err(err))
		return nil, fmt.Errorf("failed to get today's log: %w", err)
	}

	if todayLog != nil && len(todayLog.MovieIDs) > 0 {
		validMovies := []MovieDetails{}
		for _, tmdbID := range todayLog.MovieIDs {
			details, err := s.moviesSvc.GetMovieByID(ctx, tmdbID, userID)
			if err != nil {
				log.Warn("skipping stale TMDB ID", logger.Int("tmdb_id", tmdbID))
				continue
			}
			validMovies = append(validMovies, MovieDetails{
				TMDBID:      details.TMDBID,
				Title:       details.Title,
				PosterURL:   details.PosterURL,
				BackdropURL: details.BackdropURL,
				ReleaseYear: details.ReleaseYear,
				TMDBRating:  details.TMDBRating,
				Genres:      details.Genres,
				Runtime:     details.Runtime,
				Tagline:     details.Tagline,
				Status:      details.Status,
				TotalCount:  details.TotalCount,
				LikeCount:   details.LikeCount,
				LoveCount:   details.LoveCount,
				HateCount:   details.HateCount,
				SkipCount:   details.SkipCount,
				DislikeCount: details.DislikeCount,
				UserReaction: details.UserReaction,
			})
			if len(validMovies) == 2 {
				break
			}
		}

		if len(validMovies) > 0 {
			return &GenerateResult{
				Suggestions:    validMovies,
				GenerationDate: todayLog.Date,
				Regeneration:   false,
				Finished:       false,
			}, nil
		}

		// Today's log fully stale — don't regenerate, just report no valid suggestions
		log.Warn("all today's log TMDB IDs invalid, not regenerating", logger.Int("log_size", len(todayLog.MovieIDs)))
		return &GenerateResult{
			Suggestions:    []MovieDetails{},
			GenerationDate: todayLog.Date,
			Regeneration:   false,
			Finished:       true,
			Message:        "No valid suggestions available",
		}, nil
	}

	// Step 3: No existing valid log — generate fresh
	movieIDs, err := s.generateNewSuggestions(ctx, userID)
	if err != nil {
		if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "RESOURCE_EXHAUSTED") || strings.Contains(err.Error(), "quota") {
			log.Warn("gemini quota exceeded, returning graceful response")
			return &GenerateResult{
				Suggestions:    []MovieDetails{},
				GenerationDate: time.Now().Format("2006-01-02"),
				Regeneration:   true,
				Finished:       true,
				Message:        "Suggestion quota exhausted, try again later",
				IsExhausted:    true,
			}, nil
		}
		return nil, err
	}

	// Save to today's log (overwrite stale one)
	if err := s.repo.CreateTodayLog(ctx, userID, movieIDs); err != nil {
		log.Error("failed to create today's log", logger.Err(err))
		return nil, fmt.Errorf("failed to create today's log: %w", err)
	}

	// Return first 2 valid movies
	var validMovies []MovieDetails
	for _, tmdbID := range movieIDs {
		details, err := s.moviesSvc.GetMovieByID(ctx, tmdbID, userID)
		if err != nil {
			log.Warn("skipping stale TMDB ID", logger.Int("tmdb_id", tmdbID))
			continue
		}
		validMovies = append(validMovies, MovieDetails{
			TMDBID:      details.TMDBID,
			Title:       details.Title,
			PosterURL:   details.PosterURL,
			BackdropURL: details.BackdropURL,
			ReleaseYear: details.ReleaseYear,
			TMDBRating:  details.TMDBRating,
			Genres:      details.Genres,
			Runtime:     details.Runtime,
			Tagline:     details.Tagline,
			Status:      details.Status,
			TotalCount:  details.TotalCount,
			LikeCount:   details.LikeCount,
			LoveCount:   details.LoveCount,
			HateCount:   details.HateCount,
			SkipCount:   details.SkipCount,
			DislikeCount: details.DislikeCount,
			UserReaction: details.UserReaction,
		})
		if len(validMovies) == 2 {
			break
		}
	}

	now := time.Now().Format("2006-01-02")
	if len(validMovies) == 0 {
		return &GenerateResult{
			Suggestions:    []MovieDetails{},
			GenerationDate: now,
			Regeneration:   true,
			Finished:       true,
			Message:        "No valid suggestions found",
		}, nil
	}

	return &GenerateResult{
		Suggestions:    validMovies,
		GenerationDate: now,
		Regeneration:   false,
		Finished:       false,
	}, nil
}

// GetNextMovie gets the next movie after currentTMDBID from the same log
func (s *Service) GetNextMovie(ctx context.Context, userID int64, currentTMDBID int) (*NextResult, error) {
	log := logger.WithUserID(userID).With(zap.String("operation", "GetNextMovie"))

	// We need to find the log that contains currentTMDBID
	// Check today's log first, then old logs
	var targetLog *GenerationLog

	todayLog, err := s.repo.GetTodayLog(ctx, userID)
	if err != nil {
		log.Error("failed to get today's log", logger.Err(err))
		return nil, fmt.Errorf("failed to get today's log: %w", err)
	}

	if todayLog != nil {
		for _, id := range todayLog.MovieIDs {
			if id == currentTMDBID {
				targetLog = todayLog
				break
			}
		}
	}

	// If not in today's log, find old log containing this tmdb_id
	if targetLog == nil {
		oldLog, err := s.repo.FindOldLogWithMovieIDs(ctx, userID)
		if err != nil {
			log.Error("failed to find old log", logger.Err(err))
			return nil, fmt.Errorf("failed to find old log: %w", err)
		}
		if oldLog != nil {
			for _, id := range oldLog.MovieIDs {
				if id == currentTMDBID {
					targetLog = oldLog
					break
				}
			}
		}
	}

	if targetLog == nil {
		return nil, fmt.Errorf("movie not found in any active log")
	}

	// Determine if this is an old log
	now := time.Now().Format("2006-01-02")
	isOldLog := targetLog.Date < now

	// Find next movie after currentTMDBID
	nextTMDBID, err := s.repo.GetNextTMDBID(ctx, userID, targetLog.Date, currentTMDBID)
	if err != nil {
		log.Error("failed to get next tmdb id", logger.Err(err))
		return nil, fmt.Errorf("failed to get next movie: %w", err)
	}

	// If nextTMDBID is nil, this is the last movie in the log
	// We still need to return the current movie's details
	if nextTMDBID == nil {
		// Get details for the current movie (the one user is on)
		currentMovieDetails, err := s.getMovieDetails(ctx, []int{currentTMDBID}, userID)
		if err != nil {
			log.Error("failed to get current movie details", logger.Err(err))
			return nil, fmt.Errorf("failed to get current movie details: %w", err)
		}

		return &NextResult{
			Suggestion:   &currentMovieDetails[0],
			NextTMDBID:   nil,
			HasMore:      false,
			Regeneration: isOldLog, // if old log, frontend should regenerate
			Finished:     true,
			Message:      "You've completed all suggestions for this session",
		}, nil
	}

	// Get full details for the next movie
	movieDetails, err := s.getMovieDetails(ctx, []int{*nextTMDBID}, userID)
	if err != nil {
		log.Error("failed to get next movie details", logger.Err(err))
		return nil, fmt.Errorf("failed to get next movie details: %w", err)
	}

	if len(movieDetails) == 0 {
		log.Error("movie details empty after fetch", zap.Int("tmdbID", *nextTMDBID))
		return nil, fmt.Errorf("movie with TMDB ID %d not found", *nextTMDBID)
	}

	return &NextResult{
		Suggestion:   &movieDetails[0],
		NextTMDBID:   nextTMDBID,
		HasMore:      true,
		Regeneration: false,
		Finished:     false,
	}, nil
}

func (s *Service) getMovieDetails(ctx context.Context, tmdbIDs []int, userID int64) ([]MovieDetails, error) {
	var movieDetails []MovieDetails

	for _, tmdbID := range tmdbIDs {
		// Use movies service to get full details
		details, err := s.moviesSvc.GetMovieByID(ctx, tmdbID, userID)
		if err != nil {
			// If movie not found, skip it
			continue
		}

		movieDetails = append(movieDetails, MovieDetails{
			TMDBID:      details.TMDBID,
			Title:       details.Title,
			PosterURL:   details.PosterURL,
			BackdropURL: details.BackdropURL,
			ReleaseYear: details.ReleaseYear,
			TMDBRating:  details.TMDBRating,
			Genres:      details.Genres,
			Runtime:     details.Runtime,
			Tagline:     details.Tagline,
			Status:      details.Status,
			TotalCount:  details.TotalCount,
			LikeCount:   details.LikeCount,
			LoveCount:   details.LoveCount,
			HateCount:   details.HateCount,
			SkipCount:   details.SkipCount,
			DislikeCount: details.DislikeCount,
			UserReaction: details.UserReaction,
		})
	}

	return movieDetails, nil
}

func (s *Service) generateNewSuggestions(ctx context.Context, userID int64) ([]int, error) {
	log := logger.WithUserID(userID).With(zap.String("operation", "generateNewSuggestions"))

	// Check favorites count
	favCount, err := s.repo.GetFavoriteCount(ctx, userID)
	if err != nil {
		log.Error("failed to count favorites", logger.Err(err))
		return nil, fmt.Errorf("failed to count favorites: %w", err)
	}
	if favCount < minFavorites {
		log.Warn("insufficient favorites", logger.Int("fav_count", favCount), logger.Int("required", minFavorites))
		return nil, fmt.Errorf("at least %d favorite movies required", minFavorites)
	}

	// Get user data for prompt
	favorites, err := s.repo.GetFavoriteMovies(ctx, userID)
	if err != nil {
		log.Error("failed to get favorites", logger.Err(err))
		return nil, fmt.Errorf("failed to get favorites: %w", err)
	}

	watchlist, err := s.repo.GetWatchlistMovies(ctx, userID)
	if err != nil {
		log.Error("failed to get watchlist", logger.Err(err))
		return nil, fmt.Errorf("failed to get watchlist: %w", err)
	}

	reactions, err := s.repo.GetReactions(ctx, userID)
	if err != nil {
		log.Error("failed to get reactions", logger.Err(err))
		return nil, fmt.Errorf("failed to get reactions: %w", err)
	}

	excludedIDs, err := s.repo.BuildExcludedTMDBIDs(ctx, userID)
	if err != nil {
		log.Error("failed to build excluded IDs", logger.Err(err))
		return nil, fmt.Errorf("failed to build excluded IDs: %w", err)
	}

	userPrompt := s.buildUserPrompt(favorites, watchlist, reactions, excludedIDs)

	geminiResp, err := s.gemini.GetMovieSuggestions(ctx, systemPrompt, userPrompt)
	if err != nil {
		log.Error("gemini call failed", logger.Err(err))
		return nil, fmt.Errorf("gemini call failed: %w", err)
	}

	if len(geminiResp.Recommendations) == 0 {
		log.Error("empty recommendations from gemini")
		return nil, fmt.Errorf("no recommendations returned from AI")
	}

	if len(geminiResp.Recommendations) < numSuggestions {
		log.Warn("gemini returned fewer recommendations than requested", logger.Int("expected", numSuggestions), logger.Int("got", len(geminiResp.Recommendations)))
	}

	var movieIDs []int
	for _, rec := range geminiResp.Recommendations {
		if rec.TMDBID > 0 {
			movieIDs = append(movieIDs, rec.TMDBID)
		}
		if len(movieIDs) == numSuggestions {
			break
		}
	}

	return movieIDs, nil
}

func (s *Service) buildUserPrompt(favorites, watchlist []FavoriteMovie, reactions []Reaction, excludedIDs []int) string {
	prompt := "Provide 20 movie recommendations for this user.\n\n"
	prompt += "USER PROFILE:\n"

	if len(favorites) > 0 {
		prompt += fmt.Sprintf("- Favorite Movies (newest first): %s\n", titlesFromMovies(favorites))
	}
	if len(watchlist) > 0 {
		prompt += fmt.Sprintf("- Watchlist (newest first): %s\n", titlesFromMovies(watchlist))
	} else {
		prompt += "- Watchlist: (none)\n"
	}
	if len(reactions) > 0 {
		prompt += fmt.Sprintf("- Past Reactions: %s\n", reactionsString(reactions))
	} else {
		prompt += "- Past Reactions: (none yet)\n"
	}

	var seenTitles []string
	for _, f := range favorites {
		seenTitles = append(seenTitles, f.Title)
	}
	for _, w := range watchlist {
		seenTitles = append(seenTitles, w.Title)
	}
	for _, r := range reactions {
		seenTitles = append(seenTitles, r.Title)
	}

	prompt += "\nDO NOT RECOMMEND ANY OF THE FOLLOWING ALREADY SEEN/REACTED TITLES:\n"
	if len(seenTitles) > 0 {
		if len(seenTitles) > 50 {
			seenTitles = seenTitles[:50]
		}
		prompt += strings.Join(seenTitles, ", ") + "\n"
	} else {
		prompt += "(none)\n"
	}

	prompt += fmt.Sprintf(`
RULES:
- Recommend up to %d movies
- All must be UNREPRESENTED in favorites, watchlist, watched, and reactions
- Year must be 1970-2026
- TMDB ID must be a positive integer corresponding to the movie in TMDB
- Match reason must be 10-50 words explaining cinematic connections (style, theme, director, similar films)

OUTPUT FORMAT:
Return a JSON object with a "recommendations" array containing objects with keys: tmdb_id (integer), title (string), year (integer), genre (array of 1-3 strings), match_reason (string).
`, numSuggestions)

	return prompt
}
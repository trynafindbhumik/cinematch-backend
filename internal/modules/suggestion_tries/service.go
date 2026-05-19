package suggestion_tries

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/trynafindbhumik/cinematch-backend/internal/shared/gemini"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/logger"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/tmdb"
	"go.uber.org/zap"
)

const (
	minFavorites              = 5
	minReactions              = 20
	numSuggestions            = 5
	ErrWeeklyLimitExhausted   = "weekly limit exhausted"
	ErrMinFavoritesRequired   = "at least %d favorite movies required"
	ErrMinReactionsRequired   = "at least %d movie reactions required"
)

var errWeeklyLimitExhausted = errors.New(ErrWeeklyLimitExhausted)

func errMinReactionsFmt(required int) error {
	return fmt.Errorf(ErrMinReactionsRequired, required)
}

const systemPrompt = `You are a film recommendation expert with deep knowledge of global cinema — English, Hindi, Tamil, Telugu, Bengali, Korean, Japanese, French, German, Spanish, Chinese, and all other major film industries. You understand film history, directorial styles, acting styles, and what makes a movie memorable.

Provide exactly 5 movie recommendations based on the user's preferences. Focus on matching film styles, themes, directorial approaches, and cinematic elements rather than psychological interpretations.

Provide recommendations ONLY. Do not ask questions. Do not explain your reasoning process. Just output the JSON.`

type Service struct {
	repo       *Repository
	tmdbClient *tmdb.Client
	gemini     *gemini.Client
}

func NewService(repo *Repository, tmdbClient *tmdb.Client, geminiClient *gemini.Client) *Service {
	return &Service{
		repo:       repo,
		tmdbClient: tmdbClient,
		gemini:     geminiClient,
	}
}

type GenerateResult struct {
	WeekStart      string
	TryNumber      int
	Suggestions    []Movie
	GeneratedAt    string
	RemainingTries int
}

func (s *Service) GenerateSuggestions(ctx context.Context, userID int64) (*GenerateResult, error) {
	log := logger.WithUserID(userID).With(zap.String("operation", "GenerateSuggestions"))

	weekStart := GetWeekStart(time.Now())
	log = log.With(zap.String("week_start", weekStart))

	currentIndex, err := s.repo.GetCurrentSuggestionForWeek(ctx, userID, weekStart)
	if err != nil {
		log.Error("failed to get current suggestion", logger.Err(err))
		return nil, fmt.Errorf("failed to get current suggestion: %w", err)
	}

	if currentIndex >= maxTries {
		log.Warn("weekly limit exhausted", logger.Int("current_index", currentIndex))
		return nil, errWeeklyLimitExhausted
	}

	newIndex := currentIndex + 1

	favCount, err := s.repo.GetFavoriteCount(ctx, userID)
	if err != nil {
		log.Error("failed to count favorites", logger.Err(err))
		return nil, fmt.Errorf("failed to count favorites: %w", err)
	}
	if favCount < minFavorites {
		log.Warn("insufficient favorites", logger.Int("fav_count", favCount), logger.Int("required", minFavorites))
		return nil, fmt.Errorf(ErrMinFavoritesRequired, minFavorites)
	}

	reactionCount, err := s.repo.GetReactionCount(ctx, userID)
	if err != nil {
		log.Error("failed to count reactions", logger.Err(err))
		return nil, fmt.Errorf("failed to count reactions: %w", err)
	}
	if reactionCount < minReactions {
		log.Warn("insufficient reactions", logger.Int("reaction_count", reactionCount), logger.Int("required", minReactions))
		return nil, errMinReactionsFmt(minReactions)
	}

	suggestionID, err := s.repo.CreateSuggestion(ctx, userID, weekStart, newIndex)
	if err != nil {
		log.Error("failed to create suggestion", logger.Err(err))
		return nil, fmt.Errorf("failed to create suggestion: %w", err)
	}

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

	geminiResp, err := s.gemini.GetWeeklyMovieSuggestions(ctx, systemPrompt, userPrompt)
	if err != nil {
		log.Error("gemini call failed", logger.Err(err))
		return nil, fmt.Errorf("gemini call failed: %w", err)
	}

	if len(geminiResp.Recommendations) != numSuggestions {
		log.Error("unexpected recommendation count",
			logger.Int("expected", numSuggestions),
			logger.Int("got", len(geminiResp.Recommendations)))
		return nil, fmt.Errorf("expected %d suggestions, got %d", numSuggestions, len(geminiResp.Recommendations))
	}

	suggestions := make([]Movie, 0, numSuggestions)

	for _, rec := range geminiResp.Recommendations {
		details, err := s.tmdbClient.GetMovieDetails(rec.TMDBID)
		if err != nil {
			log.Warn("failed to get TMDB details, using AI data",
				logger.Int("tmdb_id", rec.TMDBID), logger.Err(err))
			posterURL := tmdb.PosterURL("")
			suggestions = append(suggestions, Movie{
				TMDBID:      rec.TMDBID,
				Title:       rec.Title,
				PosterURL:   posterURL,
				Genres:      rec.Genre,
				ReleaseYear: rec.Year,
				TMDBRating:  0,
				MatchReason: rec.MatchReason,
			})
			continue
		}

		genreNames := make([]string, len(details.Genres))
		for j, g := range details.Genres {
			genreNames[j] = g.Name
		}
		posterURL := tmdb.PosterURL(details.PosterPath)
		backdropURL := tmdb.BackdropURL(details.BackdropPath)
		releaseYear := 0
		if details.ReleaseDate != "" {
			t, _ := time.Parse("2006-01-02", details.ReleaseDate)
			releaseYear = t.Year()
		}
		tmdbRating := int(details.VoteAverage * 10)

		movieID, err := s.repo.UpsertMovie(ctx, rec.TMDBID, details.Title, posterURL, backdropURL, genreNames, releaseYear, tmdbRating)
		if err != nil {
			log.Warn("failed to upsert movie", logger.Int("tmdb_id", rec.TMDBID), logger.Err(err))
		}

		if err := s.repo.CreateSuggestionMovie(ctx, suggestionID, movieID); err != nil {
			log.Error("failed to create suggestion movie", logger.Err(err))
		}

		suggestions = append(suggestions, Movie{
			TMDBID:      rec.TMDBID,
			Title:       details.Title,
			PosterURL:   posterURL,
			BackdropURL: backdropURL,
			Genres:      genreNames,
			ReleaseYear: releaseYear,
			TMDBRating:  tmdbRating,
			MatchReason: rec.MatchReason,
		})
	}

	remainingTries := maxTries - newIndex

	return &GenerateResult{
		WeekStart:      weekStart,
		TryNumber:      newIndex,
		Suggestions:    suggestions,
		GeneratedAt:    time.Now().Format(time.RFC3339),
		RemainingTries: remainingTries,
	}, nil
}

func (s *Service) buildUserPrompt(favorites, watchlist []FavoriteMovie, reactions []Reaction, excludedIDs []int) string {
	prompt := "Provide 5 movie recommendations for this user.\n\n"
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

	prompt += "\nEXCLUDED TMDB IDs (already seen/reacted): "
	if len(excludedIDs) > 0 {
		for i, id := range excludedIDs {
			if i > 0 {
				prompt += ", "
			}
			prompt += fmt.Sprintf("%d", id)
		}
	} else {
		prompt += "(none)"
	}
	prompt += "\n"

	prompt += fmt.Sprintf(`
RULES:
- Recommend exactly %d movies
- All must be UNREPRESENTED in favorites, watchlist, watched, and reactions
- Year must be 1970-2026
- TMDB ID must be a positive integer
- Match reason must be 10-50 words explaining cinematic connections (style, theme, director, similar films)

OUTPUT FORMAT:
Return a JSON object with a "recommendations" array containing exactly %d objects with keys: tmdb_id (integer), title (string), year (integer), genre (array of 1-3 strings), match_reason (string).
`, numSuggestions, numSuggestions)

	return prompt
}
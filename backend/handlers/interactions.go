package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/harshilc/cinematch-backend/db"
	"github.com/harshilc/cinematch-backend/middleware"
)

// interactionRequest is the expected POST body for toggling a user signal.
type interactionRequest struct {
	MovieID string `json:"movie_id"`
	Type    string `json:"type"`
}

// interactionStateResponse returns all active interactions and rating for a movie.
type interactionStateResponse struct {
	Interactions []string `json:"interactions"`
	Rating       *int     `json:"rating"`
}

// ratingRequest is the expected PUT body for recording a rating.
type ratingRequest struct {
	MovieID string `json:"movie_id"`
	Score   int    `json:"score"`
}

var validInteractionTypes = map[string]bool{
	"like": true, "dislike": true, "watch": true, "skip": true,
}

// sentimentTypes are mutually exclusive: setting "like" removes "dislike" and vice versa.
var sentimentOpposite = map[string]string{
	"like":    "dislike",
	"dislike": "like",
}

const maxInteractionsPerUser = 500

// ToggleInteraction handles POST /interactions with toggle semantics.
// If the interaction exists, it removes it. If it doesn't exist, it creates it.
// Like and dislike are mutually exclusive: toggling one removes the other.
func ToggleInteraction(querier DBQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		var body interactionRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		body.MovieID = sanitizeString(body.MovieID)
		body.Type = sanitizeString(body.Type)

		if !isValidUUID(body.MovieID) {
			writeError(w, http.StatusBadRequest, "movie_id must be a valid UUID")
			return
		}
		if !validInteractionTypes[body.Type] {
			writeError(w, http.StatusBadRequest, "type must be one of: like, dislike, watch, skip")
			return
		}

		existing, err := querier.GetUserMovieInteractions(r.Context(), userID, body.MovieID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check interaction state")
			return
		}

		activeTypes := make(map[string]bool, len(existing))
		for _, row := range existing {
			activeTypes[row.Type] = true
		}

		// Toggle off: interaction already exists, remove it.
		if activeTypes[body.Type] {
			if err := querier.DeleteInteraction(r.Context(), userID, body.MovieID, body.Type); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to remove interaction")
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"action": "removed", "type": body.Type})
			return
		}

		// Enforce per-user total cap before inserting.
		totalCount, err := querier.CountUserInteractions(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check interaction limits")
			return
		}
		if totalCount >= maxInteractionsPerUser {
			writeError(w, http.StatusTooManyRequests, "interaction limit reached (500 max per account)")
			return
		}

		// Mutual exclusivity: remove the opposite sentiment if present.
		if opposite, hasSentiment := sentimentOpposite[body.Type]; hasSentiment && activeTypes[opposite] {
			if err := querier.DeleteInteraction(r.Context(), userID, body.MovieID, opposite); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to update interaction")
				return
			}
		}

		// Toggle on: insert the new interaction.
		if err := querier.UpsertInteraction(r.Context(), db.InteractionInsert{
			UserID:  userID,
			MovieID: body.MovieID,
			Type:    body.Type,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to record interaction")
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"action": "added", "type": body.Type})
	}
}

// GetMovieInteractionState handles GET /interactions?movie_id=UUID
// Returns the user's active interactions and rating for a specific movie.
func GetMovieInteractionState(querier DBQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		movieID := sanitizeString(r.URL.Query().Get("movie_id"))
		if !isValidUUID(movieID) {
			writeError(w, http.StatusBadRequest, "movie_id must be a valid UUID")
			return
		}

		interactions, err := querier.GetUserMovieInteractions(r.Context(), userID, movieID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to fetch interactions")
			return
		}

		types := make([]string, 0, len(interactions))
		for _, row := range interactions {
			types = append(types, row.Type)
		}

		rating, err := querier.GetUserMovieRating(r.Context(), userID, movieID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to fetch rating")
			return
		}

		resp := interactionStateResponse{
			Interactions: types,
		}
		if rating != nil {
			resp.Rating = &rating.Score
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

// RecordRating handles PUT /ratings
// Upserts a 1-10 star rating for a movie. Send score=0 to delete the rating.
func RecordRating(querier DBQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		var body ratingRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		body.MovieID = sanitizeString(body.MovieID)

		if !isValidUUID(body.MovieID) {
			writeError(w, http.StatusBadRequest, "movie_id must be a valid UUID")
			return
		}

		// Score 0 means "clear my rating".
		if body.Score == 0 {
			if err := querier.DeleteRating(r.Context(), userID, body.MovieID); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to remove rating")
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"action": "removed"})
			return
		}

		if body.Score < 1 || body.Score > 10 {
			writeError(w, http.StatusBadRequest, "score must be between 1 and 10")
			return
		}

		if err := querier.UpsertRating(r.Context(), db.RatingUpsert{
			UserID:  userID,
			MovieID: body.MovieID,
			Score:   body.Score,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save rating")
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"action": "saved"})
	}
}

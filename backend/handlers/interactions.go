package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/harshilc/cinematch-backend/db"
	"github.com/harshilc/cinematch-backend/middleware"
)

// interactionRequest is the expected POST body for recording a user signal.
type interactionRequest struct {
	MovieID string `json:"movie_id"`
	Type    string `json:"type"`
}

var validInteractionTypes = map[string]bool{
	"like": true, "dislike": true, "watch": true, "skip": true,
}

const (
	maxInteractionsPerUser  = 500
	maxInteractionsPerMovie = 5
)

// RecordInteraction handles POST /interactions
// Requires a valid Supabase JWT. The user_id is taken from the token, not the request body,
// so users cannot record interactions on behalf of other users.
func RecordInteraction(querier DBQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		// Strict JSON decoding: reject unknown fields to prevent schema confusion.
		var body interactionRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		// Sanitize string inputs.
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

		// Enforce per-user total interaction cap.
		totalCount, err := querier.CountUserInteractions(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check interaction limits")
			return
		}
		if totalCount >= maxInteractionsPerUser {
			writeError(w, http.StatusTooManyRequests,
				fmt.Sprintf("interaction limit reached (%d max per account)", maxInteractionsPerUser))
			return
		}

		// Enforce per-user-per-movie interaction cap.
		movieCount, err := querier.CountUserMovieInteractions(r.Context(), userID, body.MovieID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check interaction limits")
			return
		}
		if movieCount >= maxInteractionsPerMovie {
			writeError(w, http.StatusTooManyRequests,
				fmt.Sprintf("you have already recorded %d interactions for this movie", maxInteractionsPerMovie))
			return
		}

		if err := querier.InsertInteraction(r.Context(), db.InteractionInsert{
			UserID:  userID,
			MovieID: body.MovieID,
			Type:    body.Type,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to record interaction")
			return
		}

		w.WriteHeader(http.StatusCreated)
	}
}

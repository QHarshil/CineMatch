package handlers

import (
	"encoding/json"
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

		var body interactionRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		if !isValidUUID(body.MovieID) {
			writeError(w, http.StatusBadRequest, "movie_id must be a valid UUID")
			return
		}
		if !validInteractionTypes[body.Type] {
			writeError(w, http.StatusBadRequest, "type must be one of: like, dislike, watch, skip")
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

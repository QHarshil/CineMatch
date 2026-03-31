package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/harshilc/cinematch-backend/db"
	"github.com/harshilc/cinematch-backend/handlers"
	"github.com/harshilc/cinematch-backend/middleware"
)

func TestToggleInteraction(t *testing.T) {
	validMovieID := "11111111-1111-1111-1111-111111111111"
	validUserID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	tests := []struct {
		name               string
		body               string
		injectUser         string
		existingTypes      []string // interaction types already in DB for this movie
		totalCount         int
		upsertErr          error
		wantStatus         int
		wantAction         string // "added" or "removed"
		wantDeletedType    string // if non-empty, we expect this type was deleted (opposite sentiment)
	}{
		{
			name:       "toggle on: adds like when none exists",
			body:       `{"movie_id":"` + validMovieID + `","type":"like"}`,
			injectUser: validUserID,
			wantStatus: http.StatusOK,
			wantAction: "added",
		},
		{
			name:          "toggle off: removes like when already active",
			body:          `{"movie_id":"` + validMovieID + `","type":"like"}`,
			injectUser:    validUserID,
			existingTypes: []string{"like"},
			wantStatus:    http.StatusOK,
			wantAction:    "removed",
		},
		{
			name:            "mutual exclusivity: adding like removes dislike",
			body:            `{"movie_id":"` + validMovieID + `","type":"like"}`,
			injectUser:      validUserID,
			existingTypes:   []string{"dislike"},
			wantStatus:      http.StatusOK,
			wantAction:      "added",
			wantDeletedType: "dislike",
		},
		{
			name:            "mutual exclusivity: adding dislike removes like",
			body:            `{"movie_id":"` + validMovieID + `","type":"dislike"}`,
			injectUser:      validUserID,
			existingTypes:   []string{"like", "watch"},
			wantStatus:      http.StatusOK,
			wantAction:      "added",
			wantDeletedType: "like",
		},
		{
			name:          "watch and like can coexist",
			body:          `{"movie_id":"` + validMovieID + `","type":"watch"}`,
			injectUser:    validUserID,
			existingTypes: []string{"like"},
			wantStatus:    http.StatusOK,
			wantAction:    "added",
		},
		{
			name:       "returns 401 when not authenticated",
			body:       `{"movie_id":"` + validMovieID + `","type":"like"}`,
			injectUser: "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "returns 400 for invalid movie UUID",
			body:       `{"movie_id":"not-a-uuid","type":"watch"}`,
			injectUser: validUserID,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "returns 400 for unknown interaction type",
			body:       `{"movie_id":"` + validMovieID + `","type":"love"}`,
			injectUser: validUserID,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "returns 400 for malformed JSON",
			body:       `not json`,
			injectUser: validUserID,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "returns 500 on db upsert error",
			body:       `{"movie_id":"` + validMovieID + `","type":"skip"}`,
			injectUser: validUserID,
			upsertErr:  errors.New("insert failed"),
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "returns 429 when user total cap reached",
			body:       `{"movie_id":"` + validMovieID + `","type":"like"}`,
			injectUser: validUserID,
			totalCount: 500,
			wantStatus: http.StatusTooManyRequests,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var deletedTypes []string
			var capturedUpsert db.InteractionInsert

			existingRows := make([]db.InteractionRow, len(tc.existingTypes))
			for i, typ := range tc.existingTypes {
				existingRows[i] = db.InteractionRow{
					ID: "row-" + typ, UserID: validUserID, MovieID: validMovieID, Type: typ,
				}
			}

			q := &stubQuerier{
				getUserMovieInteractions: func(_ context.Context, _, _ string) ([]db.InteractionRow, error) {
					return existingRows, nil
				},
				countUserInteractions: func(_ context.Context, _ string) (int, error) {
					return tc.totalCount, nil
				},
				deleteInteraction: func(_ context.Context, _, _, iType string) error {
					deletedTypes = append(deletedTypes, iType)
					return nil
				},
				upsertInteraction: func(_ context.Context, i db.InteractionInsert) error {
					capturedUpsert = i
					return tc.upsertErr
				},
			}
			handler := handlers.ToggleInteraction(q)

			req := httptest.NewRequest(http.MethodPost, "/interactions", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			if tc.injectUser != "" {
				req = req.WithContext(middleware.WithUserID(req.Context(), tc.injectUser))
			}

			rec := httptest.NewRecorder()
			handler(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}

			if tc.wantAction != "" {
				var resp map[string]string
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp["action"] != tc.wantAction {
					t.Errorf("action = %q, want %q", resp["action"], tc.wantAction)
				}
			}

			if tc.wantDeletedType != "" {
				found := false
				for _, dt := range deletedTypes {
					if dt == tc.wantDeletedType {
						found = true
					}
				}
				if !found {
					t.Errorf("expected deletion of %q, deleted types = %v", tc.wantDeletedType, deletedTypes)
				}
			}

			if tc.wantAction == "added" && tc.upsertErr == nil && tc.wantStatus == http.StatusOK {
				if capturedUpsert.UserID != validUserID {
					t.Errorf("upsert userID = %q, want %q", capturedUpsert.UserID, validUserID)
				}
			}
		})
	}
}

func TestRecordRating(t *testing.T) {
	validMovieID := "11111111-1111-1111-1111-111111111111"
	validUserID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	tests := []struct {
		name       string
		body       string
		injectUser string
		upsertErr  error
		wantStatus int
		wantAction string
	}{
		{
			name:       "saves rating of 8",
			body:       `{"movie_id":"` + validMovieID + `","score":8}`,
			injectUser: validUserID,
			wantStatus: http.StatusOK,
			wantAction: "saved",
		},
		{
			name:       "clears rating with score 0",
			body:       `{"movie_id":"` + validMovieID + `","score":0}`,
			injectUser: validUserID,
			wantStatus: http.StatusOK,
			wantAction: "removed",
		},
		{
			name:       "returns 400 for score out of range",
			body:       `{"movie_id":"` + validMovieID + `","score":11}`,
			injectUser: validUserID,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "returns 400 for negative score",
			body:       `{"movie_id":"` + validMovieID + `","score":-1}`,
			injectUser: validUserID,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "returns 401 when not authenticated",
			body:       `{"movie_id":"` + validMovieID + `","score":5}`,
			injectUser: "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "returns 400 for invalid movie UUID",
			body:       `{"movie_id":"bad","score":5}`,
			injectUser: validUserID,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "returns 500 on db error",
			body:       `{"movie_id":"` + validMovieID + `","score":7}`,
			injectUser: validUserID,
			upsertErr:  errors.New("db down"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := &stubQuerier{
				upsertRating: func(_ context.Context, r db.RatingUpsert) error {
					return tc.upsertErr
				},
				deleteRating: func(_ context.Context, _, _ string) error {
					return nil
				},
			}
			handler := handlers.RecordRating(q)

			req := httptest.NewRequest(http.MethodPut, "/ratings", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			if tc.injectUser != "" {
				req = req.WithContext(middleware.WithUserID(req.Context(), tc.injectUser))
			}

			rec := httptest.NewRecorder()
			handler(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}

			if tc.wantAction != "" {
				var resp map[string]string
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp["action"] != tc.wantAction {
					t.Errorf("action = %q, want %q", resp["action"], tc.wantAction)
				}
			}
		})
	}
}

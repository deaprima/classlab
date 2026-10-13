package httpx_test

import (
	"errors"
	"net/http"
	"testing"

	"classlab/api/internal/httpx"
)

func TestMapError(t *testing.T) {
	reqID := "req_test_123"

	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "BadRequest",
			err:            httpx.ErrBadRequest,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "BAD_REQUEST",
		},
		{
			name:           "Unauthenticated",
			err:            httpx.ErrUnauthenticated,
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "UNAUTHENTICATED",
		},
		{
			name:           "Forbidden",
			err:            httpx.ErrForbidden,
			expectedStatus: http.StatusForbidden,
			expectedCode:   "FORBIDDEN",
		},
		{
			name:           "NotFound",
			err:            httpx.ErrNotFound,
			expectedStatus: http.StatusNotFound,
			expectedCode:   "NOT_FOUND",
		},
		{
			name:           "BookingConflict",
			err:            httpx.ErrBookingConflict,
			expectedStatus: http.StatusConflict,
			expectedCode:   "BOOKING_CONFLICT",
		},
		{
			name:           "InvalidState",
			err:            httpx.ErrInvalidState,
			expectedStatus: http.StatusConflict,
			expectedCode:   "INVALID_STATE",
		},
		{
			name:           "RoomInactive",
			err:            httpx.ErrRoomInactive,
			expectedStatus: http.StatusConflict,
			expectedCode:   "ROOM_INACTIVE",
		},
		{
			name:           "RoomHasActiveBookings",
			err:            httpx.ErrRoomHasActiveBookings,
			expectedStatus: http.StatusConflict,
			expectedCode:   "ROOM_HAS_ACTIVE_BOOKINGS",
		},
		{
			name:           "Validation",
			err:            httpx.ErrValidation,
			expectedStatus: http.StatusUnprocessableEntity,
			expectedCode:   "VALIDATION_ERROR",
		},
		{
			name:           "RateLimited",
			err:            httpx.ErrRateLimited,
			expectedStatus: http.StatusTooManyRequests,
			expectedCode:   "RATE_LIMITED",
		},
		{
			name:           "Unknown Internal Error",
			err:            errors.New("sql: connection pool exhausted"),
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   "INTERNAL_SERVER_ERROR",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, env := httpx.MapError(tc.err, reqID)

			if status != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d", tc.expectedStatus, status)
			}
			if env.Error.Code != tc.expectedCode {
				t.Fatalf("expected code %s, got %s", tc.expectedCode, env.Error.Code)
			}
			if env.Error.RequestID != reqID {
				t.Fatalf("expected request ID %s, got %s", reqID, env.Error.RequestID)
			}
		})
	}
}

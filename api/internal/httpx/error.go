package httpx

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"
)

// PGLockNotAvailable is the SQLSTATE raised when lock_timeout expires.
const PGLockNotAvailable = "55P03"

// Standard Domain Errors
var (
	ErrBadRequest            = errors.New("bad request")
	ErrUnauthenticated       = errors.New("unauthenticated")
	ErrForbidden             = errors.New("forbidden")
	ErrNotFound              = errors.New("resource not found")
	ErrBookingConflict       = errors.New("booking conflict")
	ErrInvalidState          = errors.New("invalid state transition")
	ErrRoomInactive          = errors.New("room is inactive")
	ErrRoomHasActiveBookings = errors.New("room has active bookings")
	ErrValidation            = errors.New("validation error")
	ErrRateLimited           = errors.New("rate limited")
	ErrInternal              = errors.New("internal server error")
)

type ErrorDetail struct {
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
	Kind    string `json:"kind,omitempty"`
	Status  string `json:"status,omitempty"`
	StartAt string `json:"start_at,omitempty"`
	EndAt   string `json:"end_at,omitempty"`
}

type ErrorBody struct {
	Code      string        `json:"code"`
	Message   string        `json:"message"`
	Details   []ErrorDetail `json:"details"`
	RequestID string        `json:"request_id"`
}

type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// CustomDomainError allows services to attach specific code, message, and details.
type CustomDomainError struct {
	Code       string
	Message    string
	StatusCode int
	Details    []ErrorDetail
	Err        error
}

func (e *CustomDomainError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

func (e *CustomDomainError) Unwrap() error {
	return e.Err
}

func NewCustomError(statusCode int, code, message string, details []ErrorDetail, err error) *CustomDomainError {
	return &CustomDomainError{
		StatusCode: statusCode,
		Code:       code,
		Message:    message,
		Details:    details,
		Err:        err,
	}
}

// MapError translates domain errors into HTTP status and ErrorBody.
func MapError(err error, requestID string) (int, ErrorEnvelope) {
	if details := []ErrorDetail{}; err != nil {
		var customErr *CustomDomainError
		if errors.As(err, &customErr) {
			dets := customErr.Details
			if dets == nil {
				dets = []ErrorDetail{}
			}
			return customErr.StatusCode, ErrorEnvelope{
				Error: ErrorBody{
					Code:      customErr.Code,
					Message:   customErr.Message,
					Details:   dets,
					RequestID: requestID,
				},
			}
		}

		// A statement gave up waiting for a row lock (see db.New): the system is busy, not broken.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == PGLockNotAvailable {
			return http.StatusServiceUnavailable, ErrorEnvelope{
				Error: ErrorBody{
					Code:      "SERVICE_BUSY",
					Message:   "Sistem sedang sibuk. Silakan coba lagi beberapa saat lagi.",
					Details:   details,
					RequestID: requestID,
				},
			}
		}

		switch {
		case errors.Is(err, ErrBadRequest):
			return http.StatusBadRequest, ErrorEnvelope{
				Error: ErrorBody{
					Code:      "BAD_REQUEST",
					Message:   "Permintaan tidak valid atau format salah.",
					Details:   details,
					RequestID: requestID,
				},
			}
		case errors.Is(err, ErrUnauthenticated):
			return http.StatusUnauthorized, ErrorEnvelope{
				Error: ErrorBody{
					Code:      "UNAUTHENTICATED",
					Message:   "Autentikasi gagal atau sesi telah kedaluwarsa.",
					Details:   details,
					RequestID: requestID,
				},
			}
		case errors.Is(err, ErrForbidden):
			return http.StatusForbidden, ErrorEnvelope{
				Error: ErrorBody{
					Code:      "FORBIDDEN",
					Message:   "Anda tidak memiliki hak akses untuk tindakan ini.",
					Details:   details,
					RequestID: requestID,
				},
			}
		case errors.Is(err, ErrNotFound):
			return http.StatusNotFound, ErrorEnvelope{
				Error: ErrorBody{
					Code:      "NOT_FOUND",
					Message:   "Data tidak ditemukan.",
					Details:   details,
					RequestID: requestID,
				},
			}
		case errors.Is(err, ErrBookingConflict):
			return http.StatusConflict, ErrorEnvelope{
				Error: ErrorBody{
					Code:      "BOOKING_CONFLICT",
					Message:   "Ruang sudah terpakai pada rentang waktu tersebut.",
					Details:   details,
					RequestID: requestID,
				},
			}
		case errors.Is(err, ErrInvalidState):
			return http.StatusConflict, ErrorEnvelope{
				Error: ErrorBody{
					Code:      "INVALID_STATE",
					Message:   "Transisi status tidak diizinkan.",
					Details:   details,
					RequestID: requestID,
				},
			}
		case errors.Is(err, ErrRoomInactive):
			return http.StatusConflict, ErrorEnvelope{
				Error: ErrorBody{
					Code:      "ROOM_INACTIVE",
					Message:   "Ruangan sedang tidak aktif dan tidak dapat digunakan.",
					Details:   details,
					RequestID: requestID,
				},
			}
		case errors.Is(err, ErrRoomHasActiveBookings):
			return http.StatusConflict, ErrorEnvelope{
				Error: ErrorBody{
					Code:      "ROOM_HAS_ACTIVE_BOOKINGS",
					Message:   "Ruangan memiliki booking aktif yang belum selesai.",
					Details:   details,
					RequestID: requestID,
				},
			}
		case errors.Is(err, ErrValidation):
			return http.StatusUnprocessableEntity, ErrorEnvelope{
				Error: ErrorBody{
					Code:      "VALIDATION_ERROR",
					Message:   "Data masukan tidak memenuhi validasi bisnis.",
					Details:   details,
					RequestID: requestID,
				},
			}
		case errors.Is(err, ErrRateLimited):
			return http.StatusTooManyRequests, ErrorEnvelope{
				Error: ErrorBody{
					Code:      "RATE_LIMITED",
					Message:   "Terlalu banyak permintaan. Silakan coba lagi beberapa saat lagi.",
					Details:   details,
					RequestID: requestID,
				},
			}
		}
	}

	// Default fallback to 500 without leaking internal/SQL details
	return http.StatusInternalServerError, ErrorEnvelope{
		Error: ErrorBody{
			Code:      "INTERNAL_SERVER_ERROR",
			Message:   "Terjadi kesalahan pada sistem.",
			Details:   []ErrorDetail{},
			RequestID: requestID,
		},
	}
}

// ValidationError builds a 422 VALIDATION_ERROR carrying per-field details.
func ValidationError(details ...ErrorDetail) *CustomDomainError {
	return NewCustomError(http.StatusUnprocessableEntity, "VALIDATION_ERROR",
		"Data masukan tidak memenuhi validasi.", details, ErrValidation)
}

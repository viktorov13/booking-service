package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"room-booking-service/internal/domain"
	"room-booking-service/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

type tokenParser interface {
	Parse(token string) (domain.AuthUser, error)
}

type Handler struct {
	service *service.Service
	tokens  tokenParser
}

func NewHandler(service *service.Service, tokens tokenParser) http.Handler {
	handler := &Handler{
		service: service,
		tokens:  tokens,
	}

	router := chi.NewRouter()
	router.Use(middleware.Recoverer)

	router.Get("/_info", handler.handleInfo)
	router.Post("/dummyLogin", handler.handleDummyLogin)

	router.Group(func(r chi.Router) {
		r.Use(handler.authMiddleware)
		r.Get("/rooms/list", handler.handleListRooms)
		r.Post("/rooms/create", handler.handleCreateRoom)
		r.Post("/rooms/{roomId}/schedule/create", handler.handleCreateSchedule)
		r.Get("/rooms/{roomId}/slots/list", handler.handleListSlots)
		r.Post("/bookings/create", handler.handleCreateBooking)
		r.Get("/bookings/list", handler.handleListBookings)
		r.Get("/bookings/my", handler.handleListMyBookings)
		r.Post("/bookings/{bookingId}/cancel", handler.handleCancelBooking)
	})

	return router
}

func (h *Handler) handleInfo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleDummyLogin(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Role string `json:"role"`
	}

	if err := decodeJSON(r, &request); err != nil {
		writeError(w, err)
		return
	}

	token, err := h.service.DummyLogin(r.Context(), request.Role)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (h *Handler) handleListRooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := h.service.ListRooms(r.Context(), actorFromContext(r.Context()))
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"rooms": rooms})
}

func (h *Handler) handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
		Capacity    *int    `json:"capacity"`
	}

	if err := decodeJSON(r, &request); err != nil {
		writeError(w, err)
		return
	}

	room, err := h.service.CreateRoom(r.Context(), actorFromContext(r.Context()), request.Name, request.Description, request.Capacity)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"room": room})
}

func (h *Handler) handleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	roomID, err := uuid.Parse(chi.URLParam(r, "roomId"))
	if err != nil {
		writeError(w, domain.InvalidRequest("roomId must be a valid UUID"))
		return
	}

	var request struct {
		ID         *string `json:"id"`
		RoomID     string  `json:"roomId"`
		DaysOfWeek []int   `json:"daysOfWeek"`
		StartTime  string  `json:"startTime"`
		EndTime    string  `json:"endTime"`
	}

	if err := decodeJSON(r, &request); err != nil {
		writeError(w, err)
		return
	}

	bodyRoomID, err := uuid.Parse(request.RoomID)
	if err != nil {
		writeError(w, domain.InvalidRequest("roomId must be a valid UUID"))
		return
	}

	schedule, err := h.service.CreateSchedule(r.Context(), actorFromContext(r.Context()), roomID, domain.Schedule{
		RoomID:     bodyRoomID,
		DaysOfWeek: request.DaysOfWeek,
		StartTime:  request.StartTime,
		EndTime:    request.EndTime,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"schedule": schedule})
}

func (h *Handler) handleListSlots(w http.ResponseWriter, r *http.Request) {
	roomID, err := uuid.Parse(chi.URLParam(r, "roomId"))
	if err != nil {
		writeError(w, domain.InvalidRequest("roomId must be a valid UUID"))
		return
	}

	date := r.URL.Query().Get("date")
	if date == "" {
		writeError(w, domain.InvalidRequest("date is required"))
		return
	}

	slots, err := h.service.ListAvailableSlots(r.Context(), actorFromContext(r.Context()), roomID, date)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"slots": slots})
}

func (h *Handler) handleCreateBooking(w http.ResponseWriter, r *http.Request) {
	var request struct {
		SlotID               string `json:"slotId"`
		CreateConferenceLink bool   `json:"createConferenceLink"`
	}

	if err := decodeJSON(r, &request); err != nil {
		writeError(w, err)
		return
	}

	slotID, err := uuid.Parse(request.SlotID)
	if err != nil {
		writeError(w, domain.InvalidRequest("slotId must be a valid UUID"))
		return
	}

	booking, err := h.service.CreateBooking(r.Context(), actorFromContext(r.Context()), slotID, request.CreateConferenceLink)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"booking": booking})
}

func (h *Handler) handleListBookings(w http.ResponseWriter, r *http.Request) {
	page, err := parseOptionalInt(r.URL.Query().Get("page"))
	if err != nil {
		writeError(w, domain.InvalidRequest("page must be a positive integer"))
		return
	}

	pageSize, err := parseOptionalInt(r.URL.Query().Get("pageSize"))
	if err != nil {
		writeError(w, domain.InvalidRequest("pageSize must be a positive integer"))
		return
	}

	bookings, pagination, err := h.service.ListBookings(r.Context(), actorFromContext(r.Context()), page, pageSize)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"bookings":   bookings,
		"pagination": pagination,
	})
}

func (h *Handler) handleListMyBookings(w http.ResponseWriter, r *http.Request) {
	bookings, err := h.service.ListMyBookings(r.Context(), actorFromContext(r.Context()))
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"bookings": bookings})
}

func (h *Handler) handleCancelBooking(w http.ResponseWriter, r *http.Request) {
	bookingID, err := uuid.Parse(chi.URLParam(r, "bookingId"))
	if err != nil {
		writeError(w, domain.InvalidRequest("bookingId must be a valid UUID"))
		return
	}

	booking, err := h.service.CancelBooking(r.Context(), actorFromContext(r.Context()), bookingID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"booking": booking})
}

type contextKey string

const authUserContextKey contextKey = "auth-user"

func (h *Handler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(header, "Bearer ") {
			writeError(w, domain.Unauthorized("missing bearer token"))
			return
		}

		token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		actor, err := h.tokens.Parse(token)
		if err != nil {
			writeError(w, domain.Unauthorized("invalid token"))
			return
		}

		ctx := context.WithValue(r.Context(), authUserContextKey, actor)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func actorFromContext(ctx context.Context) domain.AuthUser {
	actor, _ := ctx.Value(authUserContextKey).(domain.AuthUser)
	return actor
}

func decodeJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return domain.InvalidRequest("request body is required")
	}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(dst); err != nil {
		return domain.InvalidRequest("invalid request body")
	}

	return nil
}

func parseOptionalInt(value string) (int, error) {
	if value == "" {
		return 0, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}

	return parsed, nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, err error) {
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		writeJSON(w, appErr.HTTPStatus, map[string]any{
			"error": map[string]string{
				"code":    appErr.Code,
				"message": appErr.Message,
			},
		})
		return
	}

	log.Printf("internal error: %v", err)
	writeJSON(w, http.StatusInternalServerError, map[string]any{
		"error": map[string]string{
			"code":    "INTERNAL_ERROR",
			"message": "internal server error",
		},
	})
}

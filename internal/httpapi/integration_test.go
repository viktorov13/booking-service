package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"room-booking-service/internal/auth"
	"room-booking-service/internal/domain"
	"room-booking-service/internal/httpapi"
	"room-booking-service/internal/service"
	"room-booking-service/internal/testsupport"
)

func TestBookingFlow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC)
	env := newTestEnv(now)

	adminToken := mustLogin(t, env, "admin")
	userToken := mustLogin(t, env, "user")

	roomID := mustCreateRoom(t, env, adminToken)
	date := now.AddDate(0, 0, 1)
	mustCreateSchedule(t, env, adminToken, roomID, date)

	slots := mustListSlots(t, env, userToken, roomID, date.Format("2006-01-02"))
	if len(slots) == 0 {
		t.Fatal("expected at least one available slot")
	}

	booking := mustCreateBooking(t, env, userToken, slots[0].ID)
	if booking.Status != domain.BookingStatusActive {
		t.Fatalf("expected active booking, got %s", booking.Status)
	}
}

func TestCancelBookingFlow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC)
	env := newTestEnv(now)

	adminToken := mustLogin(t, env, "admin")
	userToken := mustLogin(t, env, "user")

	roomID := mustCreateRoom(t, env, adminToken)
	date := now.AddDate(0, 0, 1)
	mustCreateSchedule(t, env, adminToken, roomID, date)
	slots := mustListSlots(t, env, userToken, roomID, date.Format("2006-01-02"))
	booking := mustCreateBooking(t, env, userToken, slots[0].ID)

	cancelled := mustCancelBooking(t, env, userToken, booking.ID)
	if cancelled.Status != domain.BookingStatusCancelled {
		t.Fatalf("expected cancelled status, got %s", cancelled.Status)
	}

	cancelledAgain := mustCancelBooking(t, env, userToken, booking.ID)
	if cancelledAgain.Status != domain.BookingStatusCancelled {
		t.Fatalf("expected cancelled status on second cancel, got %s", cancelledAgain.Status)
	}
}

type testEnv struct {
	handler http.Handler
}

func newTestEnv(now time.Time) *testEnv {
	repo := testsupport.NewMemoryRepository()
	jwtManager := auth.NewJWTManager("test-secret", 24*time.Hour)
	svc := service.New(repo, jwtManager, func() time.Time { return now })
	handler := httpapi.NewHandler(svc, jwtManager)
	return &testEnv{handler: handler}
}

func mustLogin(t *testing.T, env *testEnv, role string) string {
	t.Helper()

	body := map[string]string{"role": role}
	response := struct {
		Token string `json:"token"`
	}{}
	doJSON(t, env, http.MethodPost, "/dummyLogin", "", body, &response)
	if response.Token == "" {
		t.Fatal("expected token in response")
	}

	return response.Token
}

func mustCreateRoom(t *testing.T, env *testEnv, token string) string {
	t.Helper()

	response := struct {
		Room struct {
			ID string `json:"id"`
		} `json:"room"`
	}{}
	doJSON(t, env, http.MethodPost, "/rooms/create", token, map[string]any{
		"name":        "Small room",
		"description": "Quiet",
		"capacity":    4,
	}, &response)
	if response.Room.ID == "" {
		t.Fatal("expected room id")
	}

	return response.Room.ID
}

func mustCreateSchedule(t *testing.T, env *testEnv, token, roomID string, date time.Time) {
	t.Helper()

	doJSON(t, env, http.MethodPost, "/rooms/"+roomID+"/schedule/create", token, map[string]any{
		"roomId":     roomID,
		"daysOfWeek": []int{isoWeekday(date)},
		"startTime":  "10:00",
		"endTime":    "12:00",
	}, nil)
}

func mustListSlots(t *testing.T, env *testEnv, token, roomID, date string) []domain.Slot {
	t.Helper()

	response := struct {
		Slots []domain.Slot `json:"slots"`
	}{}
	doJSON(t, env, http.MethodGet, "/rooms/"+roomID+"/slots/list?date="+date, token, nil, &response)
	return response.Slots
}

func mustCreateBooking(t *testing.T, env *testEnv, token string, slotID interface{ String() string }) domain.Booking {
	t.Helper()

	response := struct {
		Booking domain.Booking `json:"booking"`
	}{}
	doJSON(t, env, http.MethodPost, "/bookings/create", token, map[string]any{
		"slotId": slotID.String(),
	}, &response)
	return response.Booking
}

func mustCancelBooking(t *testing.T, env *testEnv, token string, bookingID interface{ String() string }) domain.Booking {
	t.Helper()

	response := struct {
		Booking domain.Booking `json:"booking"`
	}{}
	doJSON(t, env, http.MethodPost, "/bookings/"+bookingID.String()+"/cancel", token, map[string]any{}, &response)
	return response.Booking
}

func doJSON(t *testing.T, env *testEnv, method, path, token string, body any, dst any) {
	t.Helper()

	var requestBody []byte
	var err error
	if body != nil {
		requestBody, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
	} else {
		requestBody = nil
	}

	request, err := http.NewRequest(method, path, bytes.NewReader(requestBody))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	recorder := httptest.NewRecorder()
	env.handler.ServeHTTP(recorder, request)

	if recorder.Code < 200 || recorder.Code >= 300 {
		t.Fatalf("unexpected status %d for %s %s", recorder.Code, method, path)
	}

	if dst != nil {
		if err := json.NewDecoder(recorder.Body).Decode(dst); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}
}

func isoWeekday(value time.Time) int {
	weekday := int(value.Weekday())
	if weekday == 0 {
		return 7
	}

	return weekday
}

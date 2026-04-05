package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"room-booking-service/internal/domain"
)

type tokenResponse struct {
	Token string `json:"token"`
}

type roomResponse struct {
	Room domain.Room `json:"room"`
}

type slotsResponse struct {
	Slots []domain.Slot `json:"slots"`
}

func main() {
	baseURL := getenv("BASE_URL", "http://localhost:8080")
	client := &http.Client{Timeout: 10 * time.Second}

	adminToken := mustDummyLogin(client, baseURL, "admin")
	userToken := mustDummyLogin(client, baseURL, "user")

	roomOne := mustCreateRoom(client, baseURL, adminToken, "Room Seed A", "Seeded room A", 4)
	roomTwo := mustCreateRoom(client, baseURL, adminToken, "Room Seed B", "Seeded room B", 8)

	mustCreateSchedule(client, baseURL, adminToken, roomOne.ID.String(), []int{1, 2, 3, 4, 5}, "09:00", "18:00")
	mustCreateSchedule(client, baseURL, adminToken, roomTwo.ID.String(), []int{1, 2, 3, 4, 5}, "10:00", "17:00")

	tomorrow := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")
	slots := mustListSlots(client, baseURL, userToken, roomOne.ID.String(), tomorrow)
	if len(slots) > 0 {
		mustCreateBooking(client, baseURL, userToken, slots[0].ID.String())
	}

	log.Printf("seed completed using base URL %s", baseURL)
}

func mustDummyLogin(client *http.Client, baseURL, role string) string {
	response := tokenResponse{}
	mustDoJSON(client, http.MethodPost, baseURL+"/dummyLogin", "", map[string]string{"role": role}, &response)
	return response.Token
}

func mustCreateRoom(client *http.Client, baseURL, token, name, description string, capacity int) domain.Room {
	response := roomResponse{}
	mustDoJSON(client, http.MethodPost, baseURL+"/rooms/create", token, map[string]any{
		"name":        name,
		"description": description,
		"capacity":    capacity,
	}, &response)
	return response.Room
}

func mustCreateSchedule(client *http.Client, baseURL, token, roomID string, daysOfWeek []int, startTime, endTime string) {
	mustDoJSON(client, http.MethodPost, baseURL+"/rooms/"+roomID+"/schedule/create", token, map[string]any{
		"roomId":     roomID,
		"daysOfWeek": daysOfWeek,
		"startTime":  startTime,
		"endTime":    endTime,
	}, nil)
}

func mustListSlots(client *http.Client, baseURL, token, roomID, date string) []domain.Slot {
	response := slotsResponse{}
	mustDoJSON(client, http.MethodGet, baseURL+"/rooms/"+roomID+"/slots/list?date="+date, token, nil, &response)
	return response.Slots
}

func mustCreateBooking(client *http.Client, baseURL, token, slotID string) {
	mustDoJSON(client, http.MethodPost, baseURL+"/bookings/create", token, map[string]string{"slotId": slotID}, nil)
}

func mustDoJSON(client *http.Client, method, url, token string, payload any, dst any) {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			log.Fatalf("marshal payload: %v", err)
		}
		body = bytes.NewReader(raw)
	}

	request, err := http.NewRequest(method, url, body)
	if err != nil {
		log.Fatalf("create request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	response, err := client.Do(request)
	if err != nil {
		log.Fatalf("perform request: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(response.Body)
		log.Fatalf("unexpected status %d for %s %s: %s", response.StatusCode, method, url, string(bodyBytes))
	}

	if dst != nil {
		if err := json.NewDecoder(response.Body).Decode(dst); err != nil {
			log.Fatalf("decode response: %v", err)
		}
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func init() {
	log.SetFlags(0)
	log.SetPrefix("seed: ")
}

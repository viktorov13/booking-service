package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"room-booking-service/internal/domain"
	"room-booking-service/internal/service"
	"room-booking-service/internal/testsupport"

	"github.com/google/uuid"
)

type stubTokenIssuer struct{}

func (stubTokenIssuer) Generate(userID uuid.UUID, role domain.Role) (string, error) {
	return userID.String() + ":" + string(role), nil
}

func TestCreateScheduleRejectsNonAlignedTimes(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC)
	repo := testsupport.NewMemoryRepository()
	svc := service.New(repo, stubTokenIssuer{}, func() time.Time { return now })

	room := domain.Room{
		ID:        uuid.New(),
		Name:      "Room A",
		CreatedAt: now,
	}
	if _, err := repo.CreateRoom(context.Background(), room); err != nil {
		t.Fatalf("create room: %v", err)
	}

	_, err := svc.CreateSchedule(context.Background(), domain.AuthUser{ID: uuid.New(), Role: domain.RoleAdmin}, room.ID, domain.Schedule{
		RoomID:     room.ID,
		DaysOfWeek: []int{1},
		StartTime:  "09:15",
		EndTime:    "11:00",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}

	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST, got %v", err)
	}
}

func TestCreateBookingRejectsPastSlot(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	repo := testsupport.NewMemoryRepository()
	svc := service.New(repo, stubTokenIssuer{}, func() time.Time { return now })

	slot := domain.Slot{
		ID:     uuid.New(),
		RoomID: uuid.New(),
		Start:  now.Add(-time.Hour),
		End:    now.Add(-30 * time.Minute),
	}
	if err := repo.UpsertSlots(context.Background(), []domain.Slot{slot}); err != nil {
		t.Fatalf("upsert slot: %v", err)
	}

	_, err := svc.CreateBooking(context.Background(), domain.AuthUser{ID: uuid.New(), Role: domain.RoleUser}, slot.ID, false)
	if err == nil {
		t.Fatal("expected validation error")
	}

	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST, got %v", err)
	}
}

func TestListMyBookingsReturnsOnlyFutureSlots(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	repo := testsupport.NewMemoryRepository()
	svc := service.New(repo, stubTokenIssuer{}, func() time.Time { return now })

	userID := uuid.New()
	pastSlot := domain.Slot{
		ID:     uuid.New(),
		RoomID: uuid.New(),
		Start:  now.Add(-time.Hour),
		End:    now.Add(-30 * time.Minute),
	}
	futureSlot := domain.Slot{
		ID:     uuid.New(),
		RoomID: uuid.New(),
		Start:  now.Add(time.Hour),
		End:    now.Add(90 * time.Minute),
	}

	if err := repo.UpsertSlots(context.Background(), []domain.Slot{pastSlot, futureSlot}); err != nil {
		t.Fatalf("upsert slots: %v", err)
	}

	_, _ = repo.CreateBooking(context.Background(), domain.Booking{
		ID:        uuid.New(),
		SlotID:    pastSlot.ID,
		UserID:    userID,
		Status:    domain.BookingStatusActive,
		CreatedAt: now,
	})
	futureBooking, _ := repo.CreateBooking(context.Background(), domain.Booking{
		ID:        uuid.New(),
		SlotID:    futureSlot.ID,
		UserID:    userID,
		Status:    domain.BookingStatusCancelled,
		CreatedAt: now,
	})

	bookings, err := svc.ListMyBookings(context.Background(), domain.AuthUser{ID: userID, Role: domain.RoleUser})
	if err != nil {
		t.Fatalf("list my bookings: %v", err)
	}

	if len(bookings) != 1 {
		t.Fatalf("expected 1 future booking, got %d", len(bookings))
	}
	if bookings[0].ID != futureBooking.ID {
		t.Fatalf("expected booking %s, got %s", futureBooking.ID, bookings[0].ID)
	}
}

func TestCancelBookingIsIdempotent(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	repo := testsupport.NewMemoryRepository()
	svc := service.New(repo, stubTokenIssuer{}, func() time.Time { return now })

	userID := uuid.New()
	booking, err := repo.CreateBooking(context.Background(), domain.Booking{
		ID:        uuid.New(),
		SlotID:    uuid.New(),
		UserID:    userID,
		Status:    domain.BookingStatusActive,
		CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("create booking: %v", err)
	}

	cancelled, err := svc.CancelBooking(context.Background(), domain.AuthUser{ID: userID, Role: domain.RoleUser}, booking.ID)
	if err != nil {
		t.Fatalf("cancel booking: %v", err)
	}
	if cancelled.Status != domain.BookingStatusCancelled {
		t.Fatalf("expected cancelled status, got %s", cancelled.Status)
	}

	cancelledAgain, err := svc.CancelBooking(context.Background(), domain.AuthUser{ID: userID, Role: domain.RoleUser}, booking.ID)
	if err != nil {
		t.Fatalf("cancel booking second time: %v", err)
	}
	if cancelledAgain.Status != domain.BookingStatusCancelled {
		t.Fatalf("expected cancelled status on second call, got %s", cancelledAgain.Status)
	}
}

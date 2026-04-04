package testsupport

import (
	"context"
	"testing"
	"time"

	"room-booking-service/internal/domain"

	"github.com/google/uuid"
)

func TestMemoryRepositoryCreateBookingConflict(t *testing.T) {
	t.Parallel()

	repo := NewMemoryRepository()
	slotID := uuid.New()

	_, err := repo.CreateBooking(context.Background(), domain.Booking{
		ID:        uuid.New(),
		SlotID:    slotID,
		UserID:    uuid.New(),
		Status:    domain.BookingStatusActive,
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("create first booking: %v", err)
	}

	_, err = repo.CreateBooking(context.Background(), domain.Booking{
		ID:        uuid.New(),
		SlotID:    slotID,
		UserID:    uuid.New(),
		Status:    domain.BookingStatusActive,
		CreatedAt: time.Now().UTC(),
	})
	if err != domain.ErrSlotAlreadyBooked {
		t.Fatalf("expected ErrSlotAlreadyBooked, got %v", err)
	}
}

func TestMemoryRepositoryListUserFutureBookings(t *testing.T) {
	t.Parallel()

	repo := NewMemoryRepository()
	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
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
	expected, _ := repo.CreateBooking(context.Background(), domain.Booking{
		ID:        uuid.New(),
		SlotID:    futureSlot.ID,
		UserID:    userID,
		Status:    domain.BookingStatusCancelled,
		CreatedAt: now,
	})

	bookings, err := repo.ListUserFutureBookings(context.Background(), userID, now)
	if err != nil {
		t.Fatalf("list future bookings: %v", err)
	}

	if len(bookings) != 1 || bookings[0].ID != expected.ID {
		t.Fatalf("unexpected future bookings result: %+v", bookings)
	}
}

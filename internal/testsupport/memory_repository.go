package testsupport

import (
	"context"
	"sort"
	"sync"
	"time"

	"room-booking-service/internal/domain"

	"github.com/google/uuid"
)

type MemoryRepository struct {
	mu        sync.Mutex
	users     map[uuid.UUID]domain.User
	rooms     map[uuid.UUID]domain.Room
	schedules map[uuid.UUID]domain.Schedule
	slots     map[uuid.UUID]domain.Slot
	slotKeys  map[string]uuid.UUID
	bookings  map[uuid.UUID]domain.Booking
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		users:     make(map[uuid.UUID]domain.User),
		rooms:     make(map[uuid.UUID]domain.Room),
		schedules: make(map[uuid.UUID]domain.Schedule),
		slots:     make(map[uuid.UUID]domain.Slot),
		slotKeys:  make(map[string]uuid.UUID),
		bookings:  make(map[uuid.UUID]domain.Booking),
	}
}

func (r *MemoryRepository) UpsertUser(_ context.Context, user domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.users[user.ID] = user
	return nil
}

func (r *MemoryRepository) ListRooms(_ context.Context) ([]domain.Room, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	rooms := make([]domain.Room, 0, len(r.rooms))
	for _, room := range r.rooms {
		rooms = append(rooms, room)
	}

	sort.Slice(rooms, func(i, j int) bool {
		return rooms[i].CreatedAt.Before(rooms[j].CreatedAt)
	})

	return rooms, nil
}

func (r *MemoryRepository) CreateRoom(_ context.Context, room domain.Room) (domain.Room, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.rooms[room.ID] = room
	return room, nil
}

func (r *MemoryRepository) RoomExists(_ context.Context, roomID uuid.UUID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, ok := r.rooms[roomID]
	return ok, nil
}

func (r *MemoryRepository) CreateSchedule(_ context.Context, schedule domain.Schedule) (domain.Schedule, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.schedules[schedule.RoomID]; exists {
		return domain.Schedule{}, domain.ErrScheduleExists
	}

	r.schedules[schedule.RoomID] = schedule
	return schedule, nil
}

func (r *MemoryRepository) GetScheduleByRoom(_ context.Context, roomID uuid.UUID) (domain.Schedule, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	schedule, ok := r.schedules[roomID]
	return schedule, ok, nil
}

func (r *MemoryRepository) UpsertSlots(_ context.Context, slots []domain.Slot) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, slot := range slots {
		key := slotKey(slot.RoomID, slot.Start, slot.End)
		if _, exists := r.slotKeys[key]; exists {
			continue
		}

		r.slotKeys[key] = slot.ID
		r.slots[slot.ID] = slot
	}

	return nil
}

func (r *MemoryRepository) ListAvailableSlots(_ context.Context, roomID uuid.UUID, date time.Time, now time.Time) ([]domain.Slot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var slots []domain.Slot
	for _, slot := range r.slots {
		if slot.RoomID != roomID {
			continue
		}
		if slot.Start.Before(now) {
			continue
		}
		if !sameDay(slot.Start, date) {
			continue
		}
		if r.hasActiveBooking(slot.ID) {
			continue
		}
		slots = append(slots, slot)
	}

	sort.Slice(slots, func(i, j int) bool {
		return slots[i].Start.Before(slots[j].Start)
	})

	return slots, nil
}

func (r *MemoryRepository) GetSlot(_ context.Context, slotID uuid.UUID) (domain.Slot, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	slot, ok := r.slots[slotID]
	return slot, ok, nil
}

func (r *MemoryRepository) CreateBooking(_ context.Context, booking domain.Booking) (domain.Booking, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.hasActiveBooking(booking.SlotID) {
		return domain.Booking{}, domain.ErrSlotAlreadyBooked
	}

	r.bookings[booking.ID] = booking
	return booking, nil
}

func (r *MemoryRepository) ListBookings(_ context.Context, offset, limit int) ([]domain.Booking, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	bookings := make([]domain.Booking, 0, len(r.bookings))
	for _, booking := range r.bookings {
		bookings = append(bookings, booking)
	}

	sort.Slice(bookings, func(i, j int) bool {
		return bookings[i].CreatedAt.After(bookings[j].CreatedAt)
	})

	total := len(bookings)
	if offset >= total {
		return []domain.Booking{}, total, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	return bookings[offset:end], total, nil
}

func (r *MemoryRepository) ListUserFutureBookings(_ context.Context, userID uuid.UUID, now time.Time) ([]domain.Booking, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var bookings []domain.Booking
	for _, booking := range r.bookings {
		if booking.UserID != userID {
			continue
		}
		slot, ok := r.slots[booking.SlotID]
		if !ok || slot.Start.Before(now) {
			continue
		}
		bookings = append(bookings, booking)
	}

	sort.Slice(bookings, func(i, j int) bool {
		return r.slots[bookings[i].SlotID].Start.Before(r.slots[bookings[j].SlotID].Start)
	})

	return bookings, nil
}

func (r *MemoryRepository) GetBooking(_ context.Context, bookingID uuid.UUID) (domain.Booking, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	booking, ok := r.bookings[bookingID]
	return booking, ok, nil
}

func (r *MemoryRepository) UpdateBookingStatus(_ context.Context, bookingID uuid.UUID, status domain.BookingStatus) (domain.Booking, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	booking := r.bookings[bookingID]
	booking.Status = status
	r.bookings[bookingID] = booking
	return booking, nil
}

func (r *MemoryRepository) hasActiveBooking(slotID uuid.UUID) bool {
	for _, booking := range r.bookings {
		if booking.SlotID == slotID && booking.Status == domain.BookingStatusActive {
			return true
		}
	}

	return false
}

func slotKey(roomID uuid.UUID, start, end time.Time) string {
	return roomID.String() + "|" + start.UTC().Format(time.RFC3339) + "|" + end.UTC().Format(time.RFC3339)
}

func sameDay(left, right time.Time) bool {
	left = left.UTC()
	right = right.UTC()
	return left.Year() == right.Year() && left.YearDay() == right.YearDay()
}

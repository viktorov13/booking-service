package postgres

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"room-booking-service/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

//go:embed schema.sql
var schemaSQL string

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func RunMigrations(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, schemaSQL)
	return err
}

func (r *Repository) UpsertUser(ctx context.Context, user domain.User) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO users (id, email, role, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE
		SET email = EXCLUDED.email, role = EXCLUDED.role
	`, user.ID, user.Email, user.Role, user.CreatedAt.UTC())
	return err
}

func (r *Repository) ListRooms(ctx context.Context) ([]domain.Room, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, description, capacity, created_at
		FROM rooms
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []domain.Room
	for rows.Next() {
		room, err := scanRoom(rows)
		if err != nil {
			return nil, err
		}
		rooms = append(rooms, room)
	}

	return rooms, rows.Err()
}

func (r *Repository) CreateRoom(ctx context.Context, room domain.Room) (domain.Room, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO rooms (id, name, description, capacity, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, description, capacity, created_at
	`, room.ID, room.Name, room.Description, room.Capacity, room.CreatedAt.UTC())

	return scanRoom(row)
}

func (r *Repository) RoomExists(ctx context.Context, roomID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM rooms WHERE id = $1)`, roomID).Scan(&exists)
	return exists, err
}

func (r *Repository) CreateSchedule(ctx context.Context, schedule domain.Schedule) (domain.Schedule, error) {
	daysJSON, err := json.Marshal(schedule.DaysOfWeek)
	if err != nil {
		return domain.Schedule{}, err
	}

	row := r.db.QueryRowContext(ctx, `
		INSERT INTO schedules (id, room_id, days_of_week, start_time, end_time)
		VALUES ($1, $2, $3::jsonb, $4, $5)
		RETURNING id, room_id, days_of_week, start_time, end_time
	`, schedule.ID, schedule.RoomID, string(daysJSON), schedule.StartTime, schedule.EndTime)

	createdSchedule, scanErr := scanSchedule(row)
	if scanErr == nil {
		return createdSchedule, nil
	}

	var pgErr *pgconn.PgError
	if errors.As(scanErr, &pgErr) && pgErr.Code == "23505" {
		return domain.Schedule{}, domain.ErrScheduleExists
	}

	return domain.Schedule{}, scanErr
}

func (r *Repository) GetScheduleByRoom(ctx context.Context, roomID uuid.UUID) (domain.Schedule, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, room_id, days_of_week, start_time, end_time
		FROM schedules
		WHERE room_id = $1
	`, roomID)

	schedule, err := scanSchedule(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Schedule{}, false, nil
		}
		return domain.Schedule{}, false, err
	}

	return schedule, true, nil
}

func (r *Repository) UpsertSlots(ctx context.Context, slots []domain.Slot) error {
	if len(slots) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	for _, slot := range slots {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO slots (id, room_id, start_time, end_time, created_at)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (room_id, start_time, end_time) DO NOTHING
		`, slot.ID, slot.RoomID, slot.Start.UTC(), slot.End.UTC(), time.Now().UTC()); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *Repository) ListAvailableSlots(ctx context.Context, roomID uuid.UUID, date time.Time, now time.Time) ([]domain.Slot, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT s.id, s.room_id, s.start_time, s.end_time
		FROM slots s
		LEFT JOIN bookings b
			ON b.slot_id = s.id AND b.status = 'active'
		WHERE s.room_id = $1
		  AND s.start_time >= $2
		  AND s.start_time < $2 + interval '1 day'
		  AND s.start_time >= $3
		  AND b.id IS NULL
		ORDER BY s.start_time ASC
	`, roomID, startOfDay(date), now.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var slots []domain.Slot
	for rows.Next() {
		slot, err := scanSlot(rows)
		if err != nil {
			return nil, err
		}
		slots = append(slots, slot)
	}

	return slots, rows.Err()
}

func (r *Repository) GetSlot(ctx context.Context, slotID uuid.UUID) (domain.Slot, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, room_id, start_time, end_time
		FROM slots
		WHERE id = $1
	`, slotID)

	slot, err := scanSlot(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Slot{}, false, nil
		}
		return domain.Slot{}, false, err
	}

	return slot, true, nil
}

func (r *Repository) CreateBooking(ctx context.Context, booking domain.Booking) (domain.Booking, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO bookings (id, slot_id, user_id, status, conference_link, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, slot_id, user_id, status, conference_link, created_at
	`, booking.ID, booking.SlotID, booking.UserID, booking.Status, booking.ConferenceLink, booking.CreatedAt.UTC())

	createdBooking, err := scanBooking(row)
	if err == nil {
		return createdBooking, nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "idx_bookings_active_slot") {
		return domain.Booking{}, domain.ErrSlotAlreadyBooked
	}

	return domain.Booking{}, err
}

func (r *Repository) ListBookings(ctx context.Context, offset, limit int) ([]domain.Booking, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bookings`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, slot_id, user_id, status, conference_link, created_at
		FROM bookings
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var bookings []domain.Booking
	for rows.Next() {
		booking, err := scanBooking(rows)
		if err != nil {
			return nil, 0, err
		}
		bookings = append(bookings, booking)
	}

	return bookings, total, rows.Err()
}

func (r *Repository) ListUserFutureBookings(ctx context.Context, userID uuid.UUID, now time.Time) ([]domain.Booking, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT b.id, b.slot_id, b.user_id, b.status, b.conference_link, b.created_at
		FROM bookings b
		INNER JOIN slots s ON s.id = b.slot_id
		WHERE b.user_id = $1
		  AND s.start_time >= $2
		ORDER BY s.start_time ASC
	`, userID, now.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []domain.Booking
	for rows.Next() {
		booking, err := scanBooking(rows)
		if err != nil {
			return nil, err
		}
		bookings = append(bookings, booking)
	}

	return bookings, rows.Err()
}

func (r *Repository) GetBooking(ctx context.Context, bookingID uuid.UUID) (domain.Booking, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, slot_id, user_id, status, conference_link, created_at
		FROM bookings
		WHERE id = $1
	`, bookingID)

	booking, err := scanBooking(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Booking{}, false, nil
		}
		return domain.Booking{}, false, err
	}

	return booking, true, nil
}

func (r *Repository) UpdateBookingStatus(ctx context.Context, bookingID uuid.UUID, status domain.BookingStatus) (domain.Booking, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE bookings
		SET status = $2
		WHERE id = $1
		RETURNING id, slot_id, user_id, status, conference_link, created_at
	`, bookingID, status)

	return scanBooking(row)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRoom(s scanner) (domain.Room, error) {
	var room domain.Room
	var description sql.NullString
	var capacity sql.NullInt64

	if err := s.Scan(&room.ID, &room.Name, &description, &capacity, &room.CreatedAt); err != nil {
		return domain.Room{}, err
	}

	room.CreatedAt = room.CreatedAt.UTC()
	if description.Valid {
		room.Description = &description.String
	}
	if capacity.Valid {
		value := int(capacity.Int64)
		room.Capacity = &value
	}

	return room, nil
}

func scanSchedule(s scanner) (domain.Schedule, error) {
	var schedule domain.Schedule
	var daysJSON []byte
	if err := s.Scan(&schedule.ID, &schedule.RoomID, &daysJSON, &schedule.StartTime, &schedule.EndTime); err != nil {
		return domain.Schedule{}, err
	}

	if err := json.Unmarshal(daysJSON, &schedule.DaysOfWeek); err != nil {
		return domain.Schedule{}, err
	}

	return schedule, nil
}

func scanSlot(s scanner) (domain.Slot, error) {
	var slot domain.Slot
	if err := s.Scan(&slot.ID, &slot.RoomID, &slot.Start, &slot.End); err != nil {
		return domain.Slot{}, err
	}

	slot.Start = slot.Start.UTC()
	slot.End = slot.End.UTC()
	return slot, nil
}

func scanBooking(s scanner) (domain.Booking, error) {
	var booking domain.Booking
	var conferenceLink sql.NullString
	if err := s.Scan(&booking.ID, &booking.SlotID, &booking.UserID, &booking.Status, &conferenceLink, &booking.CreatedAt); err != nil {
		return domain.Booking{}, err
	}

	booking.CreatedAt = booking.CreatedAt.UTC()
	if conferenceLink.Valid {
		booking.ConferenceLink = &conferenceLink.String
	}

	return booking, nil
}

func startOfDay(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

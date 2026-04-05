package postgres

import (
	"context"
	"regexp"
	"testing"
	"time"

	"room-booking-service/internal/domain"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestCreateRoom(t *testing.T) {
	t.Parallel()

	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	room := domain.Room{
		ID:        uuid.New(),
		Name:      "Room A",
		CreatedAt: time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC),
	}

	rows := sqlmock.NewRows([]string{"id", "name", "description", "capacity", "created_at"}).
		AddRow(room.ID, room.Name, nil, nil, room.CreatedAt)

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO rooms")).
		WithArgs(room.ID, room.Name, nil, nil, room.CreatedAt.UTC()).
		WillReturnRows(rows)

	createdRoom, err := repo.CreateRoom(context.Background(), room)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}

	if createdRoom.ID != room.ID || createdRoom.Name != room.Name {
		t.Fatalf("unexpected room: %+v", createdRoom)
	}
}

func TestCreateUser(t *testing.T) {
	t.Parallel()

	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	passwordHash := "salt:hash"
	user := domain.User{
		ID:           uuid.New(),
		Email:        "user@example.com",
		Role:         domain.RoleUser,
		PasswordHash: &passwordHash,
		CreatedAt:    time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC),
	}

	rows := sqlmock.NewRows([]string{"id", "email", "role", "password_hash", "created_at"}).
		AddRow(user.ID, user.Email, user.Role, passwordHash, user.CreatedAt)

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO users")).
		WithArgs(user.ID, user.Email, user.Role, user.PasswordHash, user.CreatedAt.UTC()).
		WillReturnRows(rows)

	createdUser, err := repo.CreateUser(context.Background(), user)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if createdUser.ID != user.ID || createdUser.Email != user.Email {
		t.Fatalf("unexpected user: %+v", createdUser)
	}
	if createdUser.PasswordHash == nil || *createdUser.PasswordHash != passwordHash {
		t.Fatalf("unexpected password hash: %+v", createdUser.PasswordHash)
	}
}

func TestCreateUserConflict(t *testing.T) {
	t.Parallel()

	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	user := domain.User{
		ID:        uuid.New(),
		Email:     "user@example.com",
		Role:      domain.RoleUser,
		CreatedAt: time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC),
	}

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO users")).
		WithArgs(user.ID, user.Email, user.Role, user.PasswordHash, user.CreatedAt.UTC()).
		WillReturnError(&pgconn.PgError{Code: "23505"})

	_, err := repo.CreateUser(context.Background(), user)
	if err != domain.ErrEmailAlreadyExists {
		t.Fatalf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

func TestGetUserByEmail(t *testing.T) {
	t.Parallel()

	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	userID := uuid.New()
	passwordHash := "salt:hash"
	rows := sqlmock.NewRows([]string{"id", "email", "role", "password_hash", "created_at"}).
		AddRow(userID, "user@example.com", domain.RoleUser, passwordHash, time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, email, role, password_hash, created_at")).
		WithArgs("user@example.com").
		WillReturnRows(rows)

	user, ok, err := repo.GetUserByEmail(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("get user by email: %v", err)
	}
	if !ok || user.ID != userID {
		t.Fatalf("unexpected user: %+v", user)
	}
	if user.PasswordHash == nil || *user.PasswordHash != passwordHash {
		t.Fatalf("unexpected password hash: %+v", user.PasswordHash)
	}
}

func TestRoomExists(t *testing.T) {
	t.Parallel()

	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	roomID := uuid.New()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM rooms WHERE id = $1)")).
		WithArgs(roomID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	exists, err := repo.RoomExists(context.Background(), roomID)
	if err != nil {
		t.Fatalf("room exists: %v", err)
	}
	if !exists {
		t.Fatal("expected room to exist")
	}
}

func TestCreateScheduleConflict(t *testing.T) {
	t.Parallel()

	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	schedule := domain.Schedule{
		ID:         uuid.New(),
		RoomID:     uuid.New(),
		DaysOfWeek: []int{1, 2},
		StartTime:  "09:00",
		EndTime:    "10:00",
	}

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO schedules")).
		WithArgs(schedule.ID, schedule.RoomID, "[1,2]", schedule.StartTime, schedule.EndTime).
		WillReturnError(&pgconn.PgError{Code: "23505"})

	_, err := repo.CreateSchedule(context.Background(), schedule)
	if err != domain.ErrScheduleExists {
		t.Fatalf("expected ErrScheduleExists, got %v", err)
	}
}

func TestGetScheduleByRoom(t *testing.T) {
	t.Parallel()

	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	roomID := uuid.New()
	scheduleID := uuid.New()

	rows := sqlmock.NewRows([]string{"id", "room_id", "days_of_week", "start_time", "end_time"}).
		AddRow(scheduleID, roomID, []byte(`[1,3,5]`), "09:00", "11:00")

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, room_id, days_of_week, start_time, end_time")).
		WithArgs(roomID).
		WillReturnRows(rows)

	schedule, ok, err := repo.GetScheduleByRoom(context.Background(), roomID)
	if err != nil {
		t.Fatalf("get schedule: %v", err)
	}
	if !ok {
		t.Fatal("expected schedule to exist")
	}
	if schedule.ID != scheduleID || len(schedule.DaysOfWeek) != 3 {
		t.Fatalf("unexpected schedule: %+v", schedule)
	}
}

func TestGetScheduleByRoomNotFound(t *testing.T) {
	t.Parallel()

	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	roomID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, room_id, days_of_week, start_time, end_time")).
		WithArgs(roomID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "room_id", "days_of_week", "start_time", "end_time"}))

	_, ok, err := repo.GetScheduleByRoom(context.Background(), roomID)
	if err != nil {
		t.Fatalf("get schedule not found: %v", err)
	}
	if ok {
		t.Fatal("expected no schedule")
	}
}

func TestListAvailableSlots(t *testing.T) {
	t.Parallel()

	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	roomID := uuid.New()
	slotID := uuid.New()
	date := time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC)
	start := time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC)
	end := start.Add(30 * time.Minute)

	rows := sqlmock.NewRows([]string{"id", "room_id", "start_time", "end_time"}).
		AddRow(slotID, roomID, start, end)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT s.id, s.room_id, s.start_time, s.end_time")).
		WithArgs(roomID, date, start.Add(-time.Hour)).
		WillReturnRows(rows)

	slots, err := repo.ListAvailableSlots(context.Background(), roomID, date, start.Add(-time.Hour))
	if err != nil {
		t.Fatalf("list slots: %v", err)
	}

	if len(slots) != 1 || slots[0].ID != slotID {
		t.Fatalf("unexpected slots: %+v", slots)
	}
}

func TestGetSlot(t *testing.T) {
	t.Parallel()

	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	slotID := uuid.New()
	roomID := uuid.New()
	start := time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC)
	end := start.Add(30 * time.Minute)

	rows := sqlmock.NewRows([]string{"id", "room_id", "start_time", "end_time"}).
		AddRow(slotID, roomID, start, end)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, room_id, start_time, end_time")).
		WithArgs(slotID).
		WillReturnRows(rows)

	slot, ok, err := repo.GetSlot(context.Background(), slotID)
	if err != nil {
		t.Fatalf("get slot: %v", err)
	}
	if !ok || slot.ID != slotID {
		t.Fatalf("unexpected slot: %+v", slot)
	}
}

func TestCreateBookingConflict(t *testing.T) {
	t.Parallel()

	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	booking := domain.Booking{
		ID:        uuid.New(),
		SlotID:    uuid.New(),
		UserID:    uuid.New(),
		Status:    domain.BookingStatusActive,
		CreatedAt: time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC),
	}

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO bookings")).
		WithArgs(booking.ID, booking.SlotID, booking.UserID, booking.Status, booking.ConferenceLink, booking.CreatedAt.UTC()).
		WillReturnError(&pgconn.PgError{Code: "23505", ConstraintName: "idx_bookings_active_slot"})

	_, err := repo.CreateBooking(context.Background(), booking)
	if err != domain.ErrSlotAlreadyBooked {
		t.Fatalf("expected ErrSlotAlreadyBooked, got %v", err)
	}
}

func TestListBookings(t *testing.T) {
	t.Parallel()

	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	bookingID := uuid.New()
	rows := sqlmock.NewRows([]string{"id", "slot_id", "user_id", "status", "conference_link", "created_at"}).
		AddRow(bookingID, uuid.New(), uuid.New(), domain.BookingStatusActive, nil, time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM bookings")).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, slot_id, user_id, status, conference_link, created_at")).
		WithArgs(10, 0).
		WillReturnRows(rows)

	bookings, total, err := repo.ListBookings(context.Background(), 0, 10)
	if err != nil {
		t.Fatalf("list bookings: %v", err)
	}

	if total != 1 || len(bookings) != 1 || bookings[0].ID != bookingID {
		t.Fatalf("unexpected bookings response: total=%d bookings=%+v", total, bookings)
	}
}

func TestListUserFutureBookings(t *testing.T) {
	t.Parallel()

	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	userID := uuid.New()
	bookingID := uuid.New()
	now := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC)

	rows := sqlmock.NewRows([]string{"id", "slot_id", "user_id", "status", "conference_link", "created_at"}).
		AddRow(bookingID, uuid.New(), userID, domain.BookingStatusCancelled, nil, now)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT b.id, b.slot_id, b.user_id, b.status, b.conference_link, b.created_at")).
		WithArgs(userID, now).
		WillReturnRows(rows)

	bookings, err := repo.ListUserFutureBookings(context.Background(), userID, now)
	if err != nil {
		t.Fatalf("list user future bookings: %v", err)
	}
	if len(bookings) != 1 || bookings[0].ID != bookingID {
		t.Fatalf("unexpected future bookings: %+v", bookings)
	}
}

func TestGetBookingNotFound(t *testing.T) {
	t.Parallel()

	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	bookingID := uuid.New()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, slot_id, user_id, status, conference_link, created_at")).
		WithArgs(bookingID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "slot_id", "user_id", "status", "conference_link", "created_at"}))

	_, ok, err := repo.GetBooking(context.Background(), bookingID)
	if err != nil {
		t.Fatalf("get booking: %v", err)
	}
	if ok {
		t.Fatal("expected booking to be absent")
	}
}

func TestUpdateBookingStatus(t *testing.T) {
	t.Parallel()

	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	bookingID := uuid.New()
	row := sqlmock.NewRows([]string{"id", "slot_id", "user_id", "status", "conference_link", "created_at"}).
		AddRow(bookingID, uuid.New(), uuid.New(), domain.BookingStatusCancelled, nil, time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC))

	mock.ExpectQuery(regexp.QuoteMeta("UPDATE bookings")).
		WithArgs(bookingID, domain.BookingStatusCancelled).
		WillReturnRows(row)

	booking, err := repo.UpdateBookingStatus(context.Background(), bookingID, domain.BookingStatusCancelled)
	if err != nil {
		t.Fatalf("update booking status: %v", err)
	}
	if booking.Status != domain.BookingStatusCancelled {
		t.Fatalf("unexpected booking status: %+v", booking)
	}
}

func newMockRepository(t *testing.T) (*Repository, sqlmock.Sqlmock, func()) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}

	return New(db), mock, func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
		_ = db.Close()
	}
}

func TestRunMigrations(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(schemaSQL)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := RunMigrations(context.Background(), db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCreateBookingSuccess(t *testing.T) {
	t.Parallel()

	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	booking := domain.Booking{
		ID:        uuid.New(),
		SlotID:    uuid.New(),
		UserID:    uuid.New(),
		Status:    domain.BookingStatusActive,
		CreatedAt: time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC),
	}

	rows := sqlmock.NewRows([]string{"id", "slot_id", "user_id", "status", "conference_link", "created_at"}).
		AddRow(booking.ID, booking.SlotID, booking.UserID, booking.Status, nil, booking.CreatedAt)

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO bookings")).
		WithArgs(booking.ID, booking.SlotID, booking.UserID, booking.Status, booking.ConferenceLink, booking.CreatedAt.UTC()).
		WillReturnRows(rows)

	createdBooking, err := repo.CreateBooking(context.Background(), booking)
	if err != nil {
		t.Fatalf("create booking: %v", err)
	}
	if createdBooking.ID != booking.ID {
		t.Fatalf("unexpected booking: %+v", createdBooking)
	}
}

func TestGetBooking(t *testing.T) {
	t.Parallel()

	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	bookingID := uuid.New()
	rows := sqlmock.NewRows([]string{"id", "slot_id", "user_id", "status", "conference_link", "created_at"}).
		AddRow(bookingID, uuid.New(), uuid.New(), domain.BookingStatusActive, nil, time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, slot_id, user_id, status, conference_link, created_at")).
		WithArgs(bookingID).
		WillReturnRows(rows)

	booking, ok, err := repo.GetBooking(context.Background(), bookingID)
	if err != nil {
		t.Fatalf("get booking: %v", err)
	}
	if !ok || booking.ID != bookingID {
		t.Fatalf("unexpected booking: %+v", booking)
	}
}

func TestStartOfDay(t *testing.T) {
	t.Parallel()

	input := time.Date(2026, 4, 7, 13, 45, 0, 0, time.FixedZone("UTC+3", 3*60*60))
	result := startOfDay(input)

	expected := time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC)
	if result != expected {
		t.Fatalf("expected %v, got %v", expected, result)
	}
}

func TestGetSlotNotFound(t *testing.T) {
	t.Parallel()

	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	slotID := uuid.New()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, room_id, start_time, end_time")).
		WithArgs(slotID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "room_id", "start_time", "end_time"}))

	_, ok, err := repo.GetSlot(context.Background(), slotID)
	if err != nil {
		t.Fatalf("get slot: %v", err)
	}
	if ok {
		t.Fatal("expected slot to be absent")
	}
}

func TestListRooms(t *testing.T) {
	t.Parallel()

	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{"id", "name", "description", "capacity", "created_at"}).
		AddRow(uuid.New(), "Room A", nil, nil, time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, description, capacity, created_at")).
		WillReturnRows(rows)

	rooms, err := repo.ListRooms(context.Background())
	if err != nil {
		t.Fatalf("list rooms: %v", err)
	}
	if len(rooms) != 1 {
		t.Fatalf("expected one room, got %d", len(rooms))
	}
}

func TestUpsertUser(t *testing.T) {
	t.Parallel()

	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	user := domain.User{
		ID:        uuid.New(),
		Email:     "user@example.com",
		Role:      domain.RoleUser,
		CreatedAt: time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC),
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO users")).
		WithArgs(user.ID, user.Email, user.Role, user.PasswordHash, user.CreatedAt.UTC()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}
}

func TestUpsertSlots(t *testing.T) {
	t.Parallel()

	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	slot := domain.Slot{
		ID:     uuid.New(),
		RoomID: uuid.New(),
		Start:  time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC),
		End:    time.Date(2026, 4, 7, 10, 30, 0, 0, time.UTC),
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO slots")).
		WithArgs(slot.ID, slot.RoomID, slot.Start.UTC(), slot.End.UTC(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.UpsertSlots(context.Background(), []domain.Slot{slot}); err != nil {
		t.Fatalf("upsert slots: %v", err)
	}
}

package service

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"room-booking-service/internal/domain"

	"github.com/google/uuid"
)

const slotDuration = 30 * time.Minute

var (
	adminDummyUserID = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	userDummyUserID  = uuid.MustParse("22222222-2222-2222-2222-222222222222")
)

type Repository interface {
	UpsertUser(ctx context.Context, user domain.User) error
	ListRooms(ctx context.Context) ([]domain.Room, error)
	CreateRoom(ctx context.Context, room domain.Room) (domain.Room, error)
	RoomExists(ctx context.Context, roomID uuid.UUID) (bool, error)
	CreateSchedule(ctx context.Context, schedule domain.Schedule) (domain.Schedule, error)
	GetScheduleByRoom(ctx context.Context, roomID uuid.UUID) (domain.Schedule, bool, error)
	UpsertSlots(ctx context.Context, slots []domain.Slot) error
	ListAvailableSlots(ctx context.Context, roomID uuid.UUID, date time.Time, now time.Time) ([]domain.Slot, error)
	GetSlot(ctx context.Context, slotID uuid.UUID) (domain.Slot, bool, error)
	CreateBooking(ctx context.Context, booking domain.Booking) (domain.Booking, error)
	ListBookings(ctx context.Context, offset, limit int) ([]domain.Booking, int, error)
	ListUserFutureBookings(ctx context.Context, userID uuid.UUID, now time.Time) ([]domain.Booking, error)
	GetBooking(ctx context.Context, bookingID uuid.UUID) (domain.Booking, bool, error)
	UpdateBookingStatus(ctx context.Context, bookingID uuid.UUID, status domain.BookingStatus) (domain.Booking, error)
}

type TokenIssuer interface {
	Generate(userID uuid.UUID, role domain.Role) (string, error)
}

type Service struct {
	repo   Repository
	tokens TokenIssuer
	now    func() time.Time
}

func New(repo Repository, tokens TokenIssuer, now func() time.Time) *Service {
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}

	return &Service{
		repo:   repo,
		tokens: tokens,
		now:    now,
	}
}

func (s *Service) DummyLogin(ctx context.Context, role string) (string, error) {
	parsedRole := domain.Role(role)
	if !parsedRole.Valid() {
		return "", domain.InvalidRequest("role must be either admin or user")
	}

	user := domain.User{
		Role:      parsedRole,
		CreatedAt: s.now(),
	}

	switch parsedRole {
	case domain.RoleAdmin:
		user.ID = adminDummyUserID
		user.Email = "admin@example.com"
	case domain.RoleUser:
		user.ID = userDummyUserID
		user.Email = "user@example.com"
	}

	if err := s.repo.UpsertUser(ctx, user); err != nil {
		return "", err
	}

	token, err := s.tokens.Generate(user.ID, user.Role)
	if err != nil {
		return "", err
	}

	return token, nil
}

func (s *Service) ListRooms(ctx context.Context, actor domain.AuthUser) ([]domain.Room, error) {
	if !actor.Role.Valid() {
		return nil, domain.Unauthorized("invalid token")
	}

	return s.repo.ListRooms(ctx)
}

func (s *Service) CreateRoom(ctx context.Context, actor domain.AuthUser, name string, description *string, capacity *int) (domain.Room, error) {
	if actor.Role != domain.RoleAdmin {
		return domain.Room{}, domain.Forbidden("admin role is required")
	}

	if strings.TrimSpace(name) == "" {
		return domain.Room{}, domain.InvalidRequest("name is required")
	}

	if capacity != nil && *capacity <= 0 {
		return domain.Room{}, domain.InvalidRequest("capacity must be positive")
	}

	if description != nil {
		trimmed := strings.TrimSpace(*description)
		description = &trimmed
	}

	room := domain.Room{
		ID:          uuid.New(),
		Name:        strings.TrimSpace(name),
		Description: description,
		Capacity:    capacity,
		CreatedAt:   s.now(),
	}

	return s.repo.CreateRoom(ctx, room)
}

func (s *Service) CreateSchedule(ctx context.Context, actor domain.AuthUser, roomID uuid.UUID, schedule domain.Schedule) (domain.Schedule, error) {
	if actor.Role != domain.RoleAdmin {
		return domain.Schedule{}, domain.Forbidden("admin role is required")
	}

	if schedule.RoomID == uuid.Nil {
		return domain.Schedule{}, domain.InvalidRequest("roomId is required")
	}

	if schedule.RoomID != roomID {
		return domain.Schedule{}, domain.InvalidRequest("roomId in path and body must match")
	}

	exists, err := s.repo.RoomExists(ctx, roomID)
	if err != nil {
		return domain.Schedule{}, err
	}
	if !exists {
		return domain.Schedule{}, domain.RoomNotFound()
	}

	days, err := normalizeDays(schedule.DaysOfWeek)
	if err != nil {
		return domain.Schedule{}, err
	}

	if err := validateTimeRange(schedule.StartTime, schedule.EndTime); err != nil {
		return domain.Schedule{}, err
	}

	schedule.ID = uuid.New()
	schedule.DaysOfWeek = days

	createdSchedule, err := s.repo.CreateSchedule(ctx, schedule)
	if err != nil {
		if errors.Is(err, domain.ErrScheduleExists) {
			return domain.Schedule{}, domain.ScheduleExists()
		}
		return domain.Schedule{}, err
	}

	return createdSchedule, nil
}

func (s *Service) ListAvailableSlots(ctx context.Context, actor domain.AuthUser, roomID uuid.UUID, rawDate string) ([]domain.Slot, error) {
	if !actor.Role.Valid() {
		return nil, domain.Unauthorized("invalid token")
	}

	date, err := time.Parse("2006-01-02", rawDate)
	if err != nil {
		return nil, domain.InvalidRequest("date must be in YYYY-MM-DD format")
	}
	date = date.UTC()

	exists, err := s.repo.RoomExists(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, domain.RoomNotFound()
	}

	schedule, ok, err := s.repo.GetScheduleByRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return []domain.Slot{}, nil
	}

	slots, err := buildSlotsForDate(schedule, date)
	if err != nil {
		return nil, err
	}

	if len(slots) > 0 {
		if err := s.repo.UpsertSlots(ctx, slots); err != nil {
			return nil, err
		}
	}

	return s.repo.ListAvailableSlots(ctx, roomID, date, s.now())
}

func (s *Service) CreateBooking(ctx context.Context, actor domain.AuthUser, slotID uuid.UUID, createConferenceLink bool) (domain.Booking, error) {
	if actor.Role != domain.RoleUser {
		return domain.Booking{}, domain.Forbidden("booking is available only for users")
	}

	slot, ok, err := s.repo.GetSlot(ctx, slotID)
	if err != nil {
		return domain.Booking{}, err
	}
	if !ok {
		return domain.Booking{}, domain.SlotNotFound()
	}

	if slot.Start.Before(s.now()) {
		return domain.Booking{}, domain.InvalidRequest("cannot book a slot in the past")
	}

	booking := domain.Booking{
		ID:        uuid.New(),
		SlotID:    slotID,
		UserID:    actor.ID,
		Status:    domain.BookingStatusActive,
		CreatedAt: s.now(),
	}
	_ = createConferenceLink

	createdBooking, err := s.repo.CreateBooking(ctx, booking)
	if err != nil {
		if errors.Is(err, domain.ErrSlotAlreadyBooked) {
			return domain.Booking{}, domain.SlotAlreadyBooked()
		}
		return domain.Booking{}, err
	}

	return createdBooking, nil
}

func (s *Service) ListBookings(ctx context.Context, actor domain.AuthUser, page, pageSize int) ([]domain.Booking, domain.Pagination, error) {
	if actor.Role != domain.RoleAdmin {
		return nil, domain.Pagination{}, domain.Forbidden("admin role is required")
	}

	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	if page < 1 || pageSize < 1 || pageSize > 100 {
		return nil, domain.Pagination{}, domain.InvalidRequest("invalid pagination parameters")
	}

	offset := (page - 1) * pageSize
	bookings, total, err := s.repo.ListBookings(ctx, offset, pageSize)
	if err != nil {
		return nil, domain.Pagination{}, err
	}

	return bookings, domain.Pagination{
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

func (s *Service) ListMyBookings(ctx context.Context, actor domain.AuthUser) ([]domain.Booking, error) {
	if actor.Role != domain.RoleUser {
		return nil, domain.Forbidden("bookings are available only for users")
	}

	return s.repo.ListUserFutureBookings(ctx, actor.ID, s.now())
}

func (s *Service) CancelBooking(ctx context.Context, actor domain.AuthUser, bookingID uuid.UUID) (domain.Booking, error) {
	if actor.Role != domain.RoleUser {
		return domain.Booking{}, domain.Forbidden("booking cancellation is available only for users")
	}

	booking, ok, err := s.repo.GetBooking(ctx, bookingID)
	if err != nil {
		return domain.Booking{}, err
	}
	if !ok {
		return domain.Booking{}, domain.BookingNotFound()
	}

	if booking.UserID != actor.ID {
		return domain.Booking{}, domain.Forbidden("cannot cancel another user's booking")
	}

	if booking.Status == domain.BookingStatusCancelled {
		return booking, nil
	}

	return s.repo.UpdateBookingStatus(ctx, bookingID, domain.BookingStatusCancelled)
}

func normalizeDays(days []int) ([]int, error) {
	if len(days) == 0 {
		return nil, domain.InvalidRequest("daysOfWeek is required")
	}

	unique := make(map[int]struct{}, len(days))
	for _, day := range days {
		if day < 1 || day > 7 {
			return nil, domain.InvalidRequest("daysOfWeek values must be in range 1..7")
		}
		unique[day] = struct{}{}
	}

	result := make([]int, 0, len(unique))
	for day := range unique {
		result = append(result, day)
	}

	sort.Ints(result)
	return result, nil
}

func validateTimeRange(start, end string) error {
	startTime, err := parseClock(start)
	if err != nil {
		return err
	}

	endTime, err := parseClock(end)
	if err != nil {
		return err
	}

	if startTime >= endTime {
		return domain.InvalidRequest("startTime must be before endTime")
	}

	if startTime%30 != 0 || endTime%30 != 0 {
		return domain.InvalidRequest("time values must be aligned to 30 minutes")
	}

	return nil
}

func buildSlotsForDate(schedule domain.Schedule, date time.Time) ([]domain.Slot, error) {
	weekday := int(date.Weekday())
	if weekday == 0 {
		weekday = 7
	}

	isAvailableDay := false
	for _, day := range schedule.DaysOfWeek {
		if day == weekday {
			isAvailableDay = true
			break
		}
	}

	if !isAvailableDay {
		return []domain.Slot{}, nil
	}

	startMinutes, err := parseClock(schedule.StartTime)
	if err != nil {
		return nil, err
	}
	endMinutes, err := parseClock(schedule.EndTime)
	if err != nil {
		return nil, err
	}

	slots := make([]domain.Slot, 0, (endMinutes-startMinutes)/30)
	for current := startMinutes; current < endMinutes; current += 30 {
		start := time.Date(date.Year(), date.Month(), date.Day(), current/60, current%60, 0, 0, time.UTC)
		end := start.Add(slotDuration)
		slots = append(slots, domain.Slot{
			ID:     uuid.New(),
			RoomID: schedule.RoomID,
			Start:  start,
			End:    end,
		})
	}

	return slots, nil
}

func parseClock(value string) (int, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, domain.InvalidRequest("time must be in HH:MM format")
	}
	if len(parts[0]) < 1 || len(parts[0]) > 2 || len(parts[1]) != 2 {
		return 0, domain.InvalidRequest("time must be in HH:MM format")
	}

	hours, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, domain.InvalidRequest("time must be in HH:MM format")
	}
	minutes, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, domain.InvalidRequest("time must be in HH:MM format")
	}
	if hours < 0 || hours > 23 || minutes < 0 || minutes > 59 {
		return 0, domain.InvalidRequest("time must be in HH:MM format")
	}

	return hours*60 + minutes, nil
}

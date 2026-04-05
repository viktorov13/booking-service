package httpapi

import "room-booking-service/internal/domain"

type infoResponse struct {
	Status string `json:"status" example:"ok"`
}

type registerRequest struct {
	Email    string `json:"email" example:"user@example.com"`
	Password string `json:"password" example:"secret-password"`
	Role     string `json:"role" enums:"admin,user" example:"user"`
}

type loginRequest struct {
	Email    string `json:"email" example:"user@example.com"`
	Password string `json:"password" example:"secret-password"`
}

type dummyLoginRequest struct {
	Role string `json:"role" enums:"admin,user" example:"admin"`
}

type createRoomRequest struct {
	Name        string  `json:"name" example:"Room A"`
	Description *string `json:"description,omitempty" example:"Quiet room"`
	Capacity    *int    `json:"capacity,omitempty" example:"4"`
}

type createScheduleRequest struct {
	RoomID     string `json:"roomId" example:"11111111-1111-1111-1111-111111111111"`
	DaysOfWeek []int  `json:"daysOfWeek" example:"1,2,3,4,5"`
	StartTime  string `json:"startTime" example:"09:00"`
	EndTime    string `json:"endTime" example:"18:00"`
}

type createBookingRequest struct {
	SlotID string `json:"slotId" example:"11111111-1111-1111-1111-111111111111"`
}

type errorBody struct {
	Code    string `json:"code" example:"INVALID_REQUEST"`
	Message string `json:"message" example:"invalid request"`
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

type tokenResponse struct {
	Token string `json:"token"`
}

type userResponse struct {
	User domain.User `json:"user"`
}

type roomResponse struct {
	Room domain.Room `json:"room"`
}

type roomsResponse struct {
	Rooms []domain.Room `json:"rooms"`
}

type scheduleResponse struct {
	Schedule domain.Schedule `json:"schedule"`
}

type slotsResponse struct {
	Slots []domain.Slot `json:"slots"`
}

type bookingResponse struct {
	Booking domain.Booking `json:"booking"`
}

type bookingsListResponse struct {
	Bookings   []domain.Booking  `json:"bookings"`
	Pagination domain.Pagination `json:"pagination"`
}

type myBookingsResponse struct {
	Bookings []domain.Booking `json:"bookings"`
}

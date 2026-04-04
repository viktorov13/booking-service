package domain

import "errors"

var (
	ErrScheduleExists    = errors.New("schedule already exists")
	ErrSlotAlreadyBooked = errors.New("slot already booked")
)

type AppError struct {
	HTTPStatus int
	Code       string
	Message    string
}

func (e *AppError) Error() string {
	return e.Message
}

func InvalidRequest(message string) *AppError {
	return &AppError{HTTPStatus: 400, Code: "INVALID_REQUEST", Message: message}
}

func Unauthorized(message string) *AppError {
	return &AppError{HTTPStatus: 401, Code: "UNAUTHORIZED", Message: message}
}

func Forbidden(message string) *AppError {
	return &AppError{HTTPStatus: 403, Code: "FORBIDDEN", Message: message}
}

func RoomNotFound() *AppError {
	return &AppError{HTTPStatus: 404, Code: "ROOM_NOT_FOUND", Message: "room not found"}
}

func SlotNotFound() *AppError {
	return &AppError{HTTPStatus: 404, Code: "SLOT_NOT_FOUND", Message: "slot not found"}
}

func BookingNotFound() *AppError {
	return &AppError{HTTPStatus: 404, Code: "BOOKING_NOT_FOUND", Message: "booking not found"}
}

func ScheduleExists() *AppError {
	return &AppError{HTTPStatus: 409, Code: "SCHEDULE_EXISTS", Message: "schedule for this room already exists and cannot be changed"}
}

func SlotAlreadyBooked() *AppError {
	return &AppError{HTTPStatus: 409, Code: "SLOT_ALREADY_BOOKED", Message: "slot is already booked"}
}

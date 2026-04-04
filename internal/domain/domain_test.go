package domain

import "testing"

func TestRoleValid(t *testing.T) {
	t.Parallel()

	if !RoleAdmin.Valid() {
		t.Fatal("expected admin role to be valid")
	}
	if !RoleUser.Valid() {
		t.Fatal("expected user role to be valid")
	}
	if Role("guest").Valid() {
		t.Fatal("expected guest role to be invalid")
	}
}

func TestAppErrorConstructors(t *testing.T) {
	t.Parallel()

	err := InvalidRequest("bad request")
	if err.HTTPStatus != 400 || err.Code != "INVALID_REQUEST" || err.Message != "bad request" {
		t.Fatalf("unexpected invalid request error: %+v", err)
	}

	forbidden := Forbidden("denied")
	if forbidden.HTTPStatus != 403 || forbidden.Code != "FORBIDDEN" {
		t.Fatalf("unexpected forbidden error: %+v", forbidden)
	}

	if ScheduleExists().HTTPStatus != 409 {
		t.Fatal("expected schedule exists conflict status")
	}
	if SlotAlreadyBooked().HTTPStatus != 409 {
		t.Fatal("expected slot already booked conflict status")
	}
}

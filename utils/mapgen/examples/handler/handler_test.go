package handler_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/agilsyofian/utils/mapgen/examples/handler"
	"github.com/agilsyofian/utils/mapgen/examples/model"
)

func TestEmployeeRoundtrip(t *testing.T) {
	t.Parallel()

	src := model.Employee{
		ID:      uuid.MustParse("0193d3a2-8a6a-7c1e-b3d4-5f6a7b8c9d0e"),
		Name:    "Alice",
		HiredAt: time.Date(2023, time.April, 1, 0, 0, 0, 0, time.UTC),
		Address: model.Address{
			City:   "Tokyo",
			Street: "1-2-3 Chiyoda",
		},
		Nicknames: []string{"Ali", "A-chan"},
	}

	wire := handler.EmployeeToEmployeev1(src)
	got, err := handler.EmployeeFromEmployeev1(wire)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, src) {
		t.Errorf("roundtrip mismatch:\ngot  %+v\nwant %+v", got, src)
	}
}

func TestEmployeeFromEmployeev1Nil(t *testing.T) {
	t.Parallel()

	got, err := handler.EmployeeFromEmployeev1(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, model.Employee{}) {
		t.Errorf("got %+v, want zero value", got)
	}
}

func TestEmployeeFromEmployeev1InvalidID(t *testing.T) {
	t.Parallel()

	wire := handler.EmployeeToEmployeev1(model.Employee{})
	wire.Id = "not-a-uuid"
	if _, err := handler.EmployeeFromEmployeev1(wire); err == nil {
		t.Fatal("expected error for invalid UUID")
	}
}

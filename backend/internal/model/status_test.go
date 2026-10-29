package model

import (
	"errors"
	"slices"
	"testing"
)

func TestCheckTransitionDriverLifecycle(t *testing.T) {
	steps := [][2]string{
		{StatusAssigned, StatusInTransit},
		{StatusInTransit, StatusDelayed},
		{StatusDelayed, StatusInTransit},
		{StatusInTransit, StatusIssue},
		{StatusIssue, StatusArrived},
	}
	for _, s := range steps {
		if err := CheckTransition(s[0], s[1], ActorDriver); err != nil {
			t.Errorf("driver %s -> %s: unexpected error %v", s[0], s[1], err)
		}
	}
}

func TestDriverCannotDeliverWithoutPOD(t *testing.T) {
	err := CheckTransition(StatusArrived, StatusDelivered, ActorDriver)
	if !errors.Is(err, ErrPODRequired) {
		t.Fatalf("want ErrPODRequired, got %v", err)
	}
	if err := CheckTransition(StatusArrived, StatusDelivered, ActorPOD); err != nil {
		t.Fatalf("POD should complete an arrived shipment: %v", err)
	}
}

func TestSMSDeliveryAllowedFromTheRoad(t *testing.T) {
	if err := CheckTransition(StatusInTransit, StatusDelivered, ActorDriverSMS); err != nil {
		t.Fatalf("SMS delivery from in_transit: %v", err)
	}
	if err := CheckTransition(StatusAssigned, StatusDelivered, ActorDriverSMS); err == nil {
		t.Fatal("SMS delivery before departure should be refused")
	}
}

func TestFinalStatesAreFinal(t *testing.T) {
	for _, actor := range []Actor{ActorDriver, ActorDriverSMS, ActorDispatcher, ActorPOD} {
		for _, to := range AllStatuses {
			if to == StatusDelivered {
				continue
			}
			err := CheckTransition(StatusDelivered, to, actor)
			var te *TransitionError
			if !errors.As(err, &te) {
				t.Errorf("actor %d: delivered -> %s should be a TransitionError, got %v", actor, to, err)
			}
		}
	}
}

func TestNoRollbackToPending(t *testing.T) {
	for _, from := range ActiveStatuses {
		if err := CheckTransition(from, StatusPending, ActorDispatcher); err == nil {
			t.Errorf("%s -> pending should go through unassign, not a status change", from)
		}
	}
}

func TestSameStatusIsNoop(t *testing.T) {
	if err := CheckTransition(StatusInTransit, StatusInTransit, ActorDriver); !errors.Is(err, ErrSameStatus) {
		t.Fatalf("want ErrSameStatus, got %v", err)
	}
}

func TestDispatcherCanConfirmDelivery(t *testing.T) {
	if err := CheckTransition(StatusInTransit, StatusDelivered, ActorDispatcher); err != nil {
		t.Fatalf("dispatcher override: %v", err)
	}
	if err := CheckTransition(StatusPending, StatusDelivered, ActorDispatcher); err == nil {
		t.Fatal("a shipment nobody picked up cannot be delivered")
	}
}

func TestUnknownStatusRejected(t *testing.T) {
	if err := CheckTransition(StatusAssigned, "teleported", ActorDispatcher); err == nil {
		t.Fatal("unknown status should be rejected")
	}
}

func TestAllowedTransitionsForDriverButtons(t *testing.T) {
	got := AllowedTransitions(StatusInTransit, ActorDriver)
	want := []string{StatusDelayed, StatusArrived, StatusIssue}
	if !slices.Equal(got, want) {
		t.Errorf("AllowedTransitions = %v, want %v", got, want)
	}
	if len(AllowedTransitions(StatusDelivered, ActorDriver)) != 0 {
		t.Error("nothing should follow delivered")
	}
}

func TestCancelRules(t *testing.T) {
	if !CanCancel(StatusInTransit) || !CanCancel(StatusPending) {
		t.Error("active shipments should be cancellable")
	}
	if CanCancel(StatusDelivered) || CanCancel(StatusCancelled) {
		t.Error("final shipments should not be cancellable")
	}
	if !CanUnassign(StatusAssigned) || CanUnassign(StatusInTransit) {
		t.Error("only a shipment that has not left can be unassigned")
	}
}

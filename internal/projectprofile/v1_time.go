package projectprofile

import (
	"fmt"
	"time"
)

type closedIntervalV1 struct {
	from  time.Time
	until time.Time
}

func newClosedIntervalV1(name string, from time.Time, until time.Time) (closedIntervalV1, error) {
	if from.IsZero() {
		return closedIntervalV1{}, fmt.Errorf("%s start is required", name)
	}
	if !until.After(from) {
		return closedIntervalV1{}, fmt.Errorf("%s end must be later than start", name)
	}
	fromUTC := from.UTC()
	untilUTC := until.UTC()
	return closedIntervalV1{from: fromUTC, until: untilUTC}, nil
}

func (interval closedIntervalV1) valid() bool {
	return !interval.from.IsZero() && interval.until.After(interval.from)
}

func (interval closedIntervalV1) contains(other closedIntervalV1) bool {
	startsBeforeOrWith := !other.from.Before(interval.from)
	endsAfterOrWith := !other.until.After(interval.until)
	return startsBeforeOrWith && endsAfterOrWith
}

func (interval closedIntervalV1) containsMoment(moment time.Time) bool {
	startsBeforeOrWith := !moment.Before(interval.from)
	endsAfterOrWith := !moment.After(interval.until)
	return startsBeforeOrWith && endsAfterOrWith
}

type WorkIntervalV1 struct{ closedIntervalV1 }
type BasisObservationWindowV1 struct{ closedIntervalV1 }
type RoleAssignmentWindowV1 struct{ closedIntervalV1 }

func NewWorkIntervalV1(from time.Time, until time.Time) (WorkIntervalV1, error) {
	interval, err := newClosedIntervalV1("Work interval", from, until)
	return WorkIntervalV1{closedIntervalV1: interval}, err
}

func NewBasisObservationWindowV1(
	from time.Time,
	until time.Time,
) (BasisObservationWindowV1, error) {
	interval, err := newClosedIntervalV1("basis-observation window", from, until)
	return BasisObservationWindowV1{closedIntervalV1: interval}, err
}

func NewRoleAssignmentWindowV1(
	from time.Time,
	until time.Time,
) (RoleAssignmentWindowV1, error) {
	interval, err := newClosedIntervalV1("RoleAssignment window", from, until)
	return RoleAssignmentWindowV1{closedIntervalV1: interval}, err
}

func (interval WorkIntervalV1) From() time.Time            { return interval.from }
func (interval WorkIntervalV1) Until() time.Time           { return interval.until }
func (interval BasisObservationWindowV1) From() time.Time  { return interval.from }
func (interval BasisObservationWindowV1) Until() time.Time { return interval.until }
func (interval RoleAssignmentWindowV1) From() time.Time    { return interval.from }
func (interval RoleAssignmentWindowV1) Until() time.Time   { return interval.until }
func (interval WorkIntervalV1) valid() bool                { return interval.closedIntervalV1.valid() }
func (interval BasisObservationWindowV1) valid() bool      { return interval.closedIntervalV1.valid() }
func (interval RoleAssignmentWindowV1) valid() bool        { return interval.closedIntervalV1.valid() }
func (interval RoleAssignmentWindowV1) covers(work WorkIntervalV1) bool {
	return interval.contains(work.closedIntervalV1)
}

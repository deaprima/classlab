package bookings

import (
	"errors"

	"classlab/api/internal/httpx"
	"classlab/api/internal/roomtypes"
)

// ApprovalMode says who decides a booking: the system (AUTO) or an approver (MANUAL).
type ApprovalMode string

const (
	ModeAuto   ApprovalMode = "AUTO"
	ModeManual ApprovalMode = "MANUAL"
)

// ManualReason says why a booking needs a person (brief BR-17).
type ManualReason string

const (
	ReasonRoomPolicy       ManualReason = "ROOM_POLICY"
	ReasonOrganization     ManualReason = "ORGANIZATION"
	ReasonViolationHistory ManualReason = "VIOLATION_HISTORY"
)

// ErrRoleNotAllowed means the requester's role is not among the room type's requester_roles.
var ErrRoleNotAllowed = errors.New("requester role not allowed for this room type")

// ApprovalInput is everything the approval decision depends on. It carries no database handle and no
// clock: the decision is a pure function of these values.
type ApprovalInput struct {
	Requester httpx.Role
	Policy    roomtypes.Policy
	// ActivePoints are the requester's violation points that are not annulled and inside the window.
	ActivePoints int
	// AutoApproveMaxPoints is the AUTO_APPROVE_MAX_POINTS setting: auto-approval needs strictly fewer
	// active points than this. Zero switches auto-approval off.
	AutoApproveMaxPoints int
}

// Decision is the outcome: Reason is empty exactly when Mode is AUTO.
type Decision struct {
	Mode   ApprovalMode
	Reason ManualReason
}

// DecideApproval is the one place that decides whether a request is approved by the system or needs an
// approver (BR-17). The conditions are checked in the order of the brief's table; nothing else in the code
// branches on role or room type for this. Conflicts, quota, the block and the booking's own validity are
// checked before this is asked, so a request that reaches it is otherwise acceptable.
//
// A requester with a poor history is not refused here, only sent to a person; refusing someone above the
// block threshold is the caller's job (BOOKING_BLOCKED).
func DecideApproval(in ApprovalInput) (Decision, error) {
	if !in.Policy.CanRequest(in.Requester) {
		return Decision{}, ErrRoleNotAllowed
	}
	switch {
	case in.Requester == httpx.RoleOrganisasi:
		return Decision{ModeManual, ReasonOrganization}, nil
	case in.Policy.ApprovalPolicy != "AUTO_IF_ELIGIBLE":
		return Decision{ModeManual, ReasonRoomPolicy}, nil
	case in.AutoApproveMaxPoints <= 0:
		return Decision{ModeManual, ReasonRoomPolicy}, nil
	case in.ActivePoints >= in.AutoApproveMaxPoints:
		return Decision{ModeManual, ReasonViolationHistory}, nil
	}
	return Decision{Mode: ModeAuto}, nil
}

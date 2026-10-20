package audit

// Action names stored in audit_logs.action. They are constants, not free text, so the log can be
// filtered reliably. Add a constant here with every new administrative endpoint (agent.md section 5).
const (
	ActionUserCreated       = "USER_CREATED"
	ActionUserUpdated       = "USER_UPDATED"
	ActionUserRoleChanged   = "USER_ROLE_CHANGED"
	ActionUserDeactivated   = "USER_DEACTIVATED"
	ActionUserActivated     = "USER_ACTIVATED"
	ActionUserBlockCleared  = "USER_BLOCK_CLEARED"
	ActionUserPasswordReset = "USER_PASSWORD_RESET"
	ActionUsersImported     = "USERS_IMPORTED"

	ActionRoomCreated = "ROOM_CREATED"
	ActionRoomUpdated = "ROOM_UPDATED"
	ActionRoomDeleted = "ROOM_DELETED"

	ActionScheduleCreated = "SCHEDULE_CREATED"
	ActionScheduleUpdated = "SCHEDULE_UPDATED"
	ActionScheduleDeleted = "SCHEDULE_DELETED"

	ActionRoomTypePolicyUpdated = "ROOM_TYPE_POLICY_UPDATED"

	ActionSchedulesImported = "SCHEDULES_IMPORTED"
	ActionImportRolledBack  = "IMPORT_ROLLED_BACK"
)

// Entity types stored in audit_logs.entity_type.
const (
	EntityUser     = "USER"
	EntityRoom     = "ROOM"
	EntitySchedule = "SCHEDULE"
	EntityImport   = "IMPORT"
	EntityRoomType = "ROOM_TYPE"
)

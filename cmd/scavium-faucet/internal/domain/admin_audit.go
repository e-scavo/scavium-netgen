package domain

// AdminAuditEntry is a durable admin audit record used across service and store layers.
type AdminAuditEntry struct {
	Action    string
	Actor     string
	Target    string
	Detail    string
	CreatedAt string
}

package domain

import "time"

// LoginStatus is a user-confirmed status; CloudMatrix never stores SMS codes.
type LoginStatus string

const (
	LoginStatusUnknown             LoginStatus = "unknown"
	LoginStatusNotInstalled        LoginStatus = "not_installed"
	LoginStatusNotLoggedIn         LoginStatus = "not_logged_in"
	LoginStatusVerificationPending LoginStatus = "verification_pending"
	LoginStatusLoggedIn            LoginStatus = "logged_in"
	LoginStatusExpired             LoginStatus = "expired"
)

// AppSession stores only a coarse login state and an optional masked account label.
type AppSession struct {
	InstanceID   string      `json:"instance_id"`
	PackageName  string      `json:"package_name"`
	LoginStatus  LoginStatus `json:"login_status"`
	AccountLabel string      `json:"account_label"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

// Valid reports whether the status belongs to the persisted state machine.
func (s LoginStatus) Valid() bool {
	switch s {
	case LoginStatusUnknown, LoginStatusNotInstalled, LoginStatusNotLoggedIn,
		LoginStatusVerificationPending, LoginStatusLoggedIn, LoginStatusExpired:
		return true
	default:
		return false
	}
}

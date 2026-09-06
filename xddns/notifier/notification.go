package notifier

import (
	"github.com/iceflowre/xddns/xddns/lib"
)

// Notification represents a notification message that can be sent to a notifier.
type Notification struct {
	Reason      Reason
	UpdaterName string
	Error       error
	IPs         lib.IPs
}

// ReasonText returns a human-readable text for the given reason.
func ReasonText(reason Reason) string {
	switch reason {
	case ReasonFailedToResolve:
		return "failed to resolve IPs"
	case ReasonInfo:
		return "info"
	case ReasonUpdateSucceeded:
		return "update succeeded"
	case ReasonWarning:
		return "warning"
	case ReasonError:
		return "error"
	case ReasonUpdateFailed:
		return "update failed"
	default:
		return string(reason)
	}
}

// Reason represents the reason for a notification.
type Reason string

// IsEquivalent checks if two reasons are equivalent based on their group.
func (r Reason) IsEquivalent(other Reason) bool {
	return r.Group() == other.Group()
}

// Group returns the group of the reason, which is used to determine if two reasons are equivalent.
func (r Reason) Group() Reason {
	switch r {
	case ReasonInfo, ReasonUpdateSucceeded:
		return ReasonInfo
	case ReasonWarning:
		return ReasonWarning
	case ReasonError, ReasonFailedToResolve, ReasonUpdateFailed:
		return ReasonError
	default:
		return r
	}
}

// IsInfo returns true if the reason is informational.
func (r Reason) IsInfo() bool {
	return r.Group() == ReasonInfo
}

// IsWarning returns true if the reason is a warning.
func (r Reason) IsWarning() bool {
	return r.Group() == ReasonWarning
}

// IsError returns true if the reason is an error or a failed update.
func (r Reason) IsError() bool {
	return r.Group() == ReasonError
}

const (
	// ReasonInfo is a notification reason for informational messages.
	ReasonInfo Reason = "info"
	// ReasonUpdateSucceeded is a notification reason for successful updates.
	ReasonUpdateSucceeded Reason = "update_succeeded"
	// ReasonWarning is a notification reason for warnings.
	ReasonWarning Reason = "warning"
	// ReasonError is a notification reason for errors.
	ReasonError Reason = "error"
	// ReasonUpdateFailed is a notification reason for failed DNS updates.
	ReasonUpdateFailed Reason = "update_failed"
	// ReasonFailedToResolve is a notification reason for failed IP resolution.
	ReasonFailedToResolve Reason = "failed_to_resolve"
)

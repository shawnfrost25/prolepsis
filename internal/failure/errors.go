package failure

import "fmt"

// Store errors that the system doesn't give us

func EmptyValue(subject string) error {
	return fmt.Errorf("value %q cannot be empty", subject)
}

func RateLimit() error {
	return fmt.Errorf("vser is ratelimited, cannot perform action")
}

func Invalid(subject, reason string) error {
	return fmt.Errorf("value %q is invalid: %s", subject, reason)
}

func NotFound(subject string) error {
	return fmt.Errorf("value %q was not found", subject)
}

func Conflict(subject string) error {
	return fmt.Errorf("value %q already exists", subject)
}

func Forbidden(subject, reason string) error {
	return fmt.Errorf("action %q is forbidden: %s", subject, reason)
}

func Unauthorized(subject, reason string) error {
	return fmt.Errorf("action %q is unauthorized: %s", subject, reason)
}

func General(subject, reason string) error {
	return fmt.Errorf("operation %q failed: %s", subject, reason)
}

func InternalGeneral(subject, reason string) error {
	return fmt.Errorf("operation %q ended unexpectedly: %s", subject, reason)
}

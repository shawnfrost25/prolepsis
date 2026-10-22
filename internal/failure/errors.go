package failure

import "fmt"

// Store errors that the system doesn't give us

func EmptyValue(subject string) error {
	return fmt.Errorf("Value %q cannot be empty", subject)
}

func RateLimit() error {
	return fmt.Errorf("User is ratelimited, cannot perform action")
}

func Invalid(subject, reason string) error {
	return fmt.Errorf("Value %q is invalid: %s", subject, reason)
}

func NotFound(subject string) error {
	return fmt.Errorf("Value %q was not found", subject)
}

func Conflict(subject string) error {
	return fmt.Errorf("Value %q already exists", subject)
}

func Forbidden(subject, reason string) error {
	return fmt.Errorf("Action %q is forbidden: %s", subject, reason)
}

func Unauthorized(subject, reason string) error {
	return fmt.Errorf("Action %q is unauthorized: %s", subject, reason)
}

func General(subject, reason string) error {
	return fmt.Errorf("Operation %q failed: %s", subject, reason)
}

func InternalGeneral(subject, reason string) error {
	return fmt.Errorf("Operation %q ended unexpectedly: %s", subject, reason)
}

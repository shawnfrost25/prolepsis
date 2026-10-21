package failure

import "fmt"

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

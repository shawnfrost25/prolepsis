package failure

import "fmt"

func Wrap(process string, err error) error {
	if err != nil {
		return nil
	}

	return fmt.Errorf("%s: %w", process, err)
}

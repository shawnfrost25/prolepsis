package lib

import (
	"context"
	"errors"
	"fmt"

	"net/smtp"

	"github.com/rs/zerolog"
)

var ErrEmailCooldown = errors.New("already sent mail to the given email, return after 20 hours")

func SendEmail(
	ctx context.Context,
	logger zerolog.Logger,
	email string,
	appPassword string,
	toEmail string,
	subject string,
	message string,
) error {

	to := []string{toEmail}

	smtpAuth := smtp.PlainAuth("", email, appPassword, "smtp.gmail.com")

	msgBytes := fmt.Appendf(nil,
		"To: %s\r\n"+
			"Subject: %s\r\n"+
			"MIME-Version: 1.0\r\n"+
			"Content-Type: text/plain; charset=\"UTF-8\"\r\n"+
			"\r\n"+
			"%s",
		toEmail, subject, message,
	)

	err := smtp.SendMail("smtp.gmail.com"+":"+"587", smtpAuth, email, to, msgBytes)
	if err != nil {
		return err
	}

	logger.Info().
		Str("to", toEmail).
		Msg("successfully transmitted email package")

	return nil
}

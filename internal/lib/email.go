package lib

import (
	"context"
	"errors"
	"fmt"
	"net/smtp"
	db "prolepsis/internal/db/sqlc"
	"strings"

	"github.com/rs/zerolog"
)

func SendEmail(logger zerolog.Logger, queries *db.Queries, fromEmail string, appPassword string, toEmail string, subject string, message string) error {
	logger.Debug().
		Str("from", fromEmail).
		Str("to", toEmail).
		Msg("attempting to deliver raw SMTP email")

	subject = strings.ReplaceAll(subject, "\r", "")
	subject = strings.ReplaceAll(subject, "\n", "")

	host := "smtp.gmail.com"
	addr := host + ":587"

	auth := smtp.PlainAuth("", fromEmail, appPassword, host)

	msgFormat := fmt.Sprintf(
		"From: %s\r\n"+
			"To: %s\r\n"+
			"Subject: %s\r\n"+
			"\r\n"+
			"%s", fromEmail, toEmail, subject, message,
	)

	alreadySent, err := queries.EmailExists(context.Background(), toEmail)
	if err != nil {
		return err
	}
	// If the email already was sent once and it has cooldown on, we return a warning
	if alreadySent {
		return errors.New("already sent mail to the given email, return after 20 hours")
	}

	err = smtp.SendMail(addr, auth, fromEmail, []string{toEmail}, []byte(msgFormat))
	if err != nil {
		return err
	}

	logger.Info().
		Str("from", fromEmail).
		Str("to", toEmail).
		Msg("successfully transmitted email package")

	queries.InsertEmailCooldown(context.Background(), toEmail)
	return nil
}

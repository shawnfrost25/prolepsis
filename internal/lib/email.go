package lib

import (
	"context"
	"errors"
	"fmt"

	db "prolepsis/internal/db/sqlc"

	"github.com/resend/resend-go/v2"
	"github.com/rs/zerolog"
)

var ErrEmailCooldown = errors.New("already sent mail to the given email, return after 20 hours")

func SendLimitedEmail(
	ctx context.Context,
	logger zerolog.Logger,
	queries *db.Queries,
	apiKey string,
	toEmail string,
	subject string,
	message string,
) error {
	if queries == nil {
		return errors.New("database queries instance is nil")
	}

	alreadySent, err := queries.EmailExists(ctx, toEmail)
	if err != nil {
		return fmt.Errorf("failed to check email cooldown status: %w", err)
	}
	if alreadySent {
		return ErrEmailCooldown
	}

	client := resend.NewClient(apiKey)
	params := &resend.SendEmailRequest{
		From:    "Prolepsis <onboarding@resend.dev>",
		To:      []string{toEmail},
		Subject: subject,
		Text:    message,
	}

	_, err = client.Emails.SendWithContext(ctx, params)
	if err != nil {
		return fmt.Errorf("resend delivery failed: %w", err)
	}

	if err := queries.InsertEmailCooldown(ctx, toEmail); err != nil {
		logger.Error().Err(err).Str("to", toEmail).Msg("failed to insert email cooldown record")
		return fmt.Errorf("failed to record email cooldown: %w", err)
	}

	logger.Info().
		Str("to", toEmail).
		Msg("successfully transmitted email package and set cooldown")

	return nil
}

func SendEmail(
	ctx context.Context,
	logger zerolog.Logger,
	apiKey string,
	toEmail string,
	subject string,
	message string,
) error {
	client := resend.NewClient(apiKey)

	params := &resend.SendEmailRequest{
		From:    "Prolepsis <onboarding@resend.dev>",
		To:      []string{toEmail},
		Subject: subject,
		Text:    message,
	}

	_, err := client.Emails.SendWithContext(ctx, params)
	if err != nil {
		return fmt.Errorf("resend delivery failed: %w", err)
	}

	logger.Info().
		Str("to", toEmail).
		Msg("successfully transmitted email package")

	return nil
}

-- We save the one to who we sent the email, so we place a timeout - so nobody will flood other's email for no reaso
-- name: InsertEmailCooldown :exec
INSERT INTO sent_emails (to_email, sent_at, expires_at)
VALUES ($1, now(), now() + interval '20 hours') ON CONFLICT (to_email) DO UPDATE
SET sent_at = EXCLUDED.sent_at, expires_at = EXCLUDED.expires_at;

-- We check if somebody already sent a mail to that email, if yes, we won't send any more
-- name: EmailExists :one
SELECT EXISTS (
    SELECT 1
    FROM sent_emails
    WHERE to_email = $1 AND expires_at > now() 
);
-- Modify "pending_registrations" table
ALTER TABLE "public"."pending_registrations" DROP COLUMN "attempts", ADD COLUMN "status" text NOT NULL DEFAULT 'pending';
-- Create index "idx_pending_registrations_email" to table: "pending_registrations"
CREATE INDEX "idx_pending_registrations_email" ON "public"."pending_registrations" ("email");
-- Rename an index from "sessions_user_id_idx" to "idx_sessions_user_id"
ALTER INDEX "public"."sessions_user_id_idx" RENAME TO "idx_sessions_user_id";
-- Create "sent_emails" table
CREATE TABLE "public"."sent_emails" (
  "to_email" text NOT NULL,
  "sent_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "expires_at" timestamptz NOT NULL,
  CONSTRAINT "sent_emails_to_email_key" UNIQUE ("to_email"),
  CONSTRAINT "sent_emails_email_check" CHECK (((length(TRIM(BOTH FROM to_email)) >= 6) AND (length(TRIM(BOTH FROM to_email)) <= 254)) AND (to_email ~* '^[a-z0-9._+-]{1,64}@([a-z0-9-]{1,63}\.)+([a-z0-9]{2,18})$'::text))
);
-- Create index "idx_sent_emails_cooldown" to table: "sent_emails"
CREATE INDEX "idx_sent_emails_cooldown" ON "public"."sent_emails" ("to_email", "expires_at");

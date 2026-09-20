-- Modify "pending_deletions" table
ALTER TABLE "public"."pending_deletions" ADD COLUMN "scheduled_at" timestamptz NOT NULL;
-- Drop "pending_registrations" table
DROP TABLE "public"."pending_registrations";
-- Drop "sent_emails" table
DROP TABLE "public"."sent_emails";
-- Drop "sessions" table
DROP TABLE "public"."sessions";

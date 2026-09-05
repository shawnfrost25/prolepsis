-- Modify "pending_registrations" table
ALTER TABLE "public"."pending_registrations" ADD COLUMN "attempts" smallint NOT NULL DEFAULT 1;

-- Add value to enum type: "user_role"
ALTER TYPE "public"."user_role" ADD VALUE 'owner';
-- Create "pending_deletions" table
CREATE TABLE "public"."pending_deletions" (
  "user_id" uuid NOT NULL,
  "requested_by" uuid NOT NULL,
  "clarification" text NOT NULL,
  "requested_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "is_processed" boolean NOT NULL DEFAULT false,
  CONSTRAINT "pending_deletions_requested_by_fkey" FOREIGN KEY ("requested_by") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "pending_deletions_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "pending_deletions_clarification_check" CHECK (length(TRIM(BOTH FROM clarification)) <= 1024)
);

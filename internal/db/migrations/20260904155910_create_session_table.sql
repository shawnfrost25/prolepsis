-- Create enum type "user_role"
CREATE TYPE "public"."user_role" AS ENUM ('user', 'admin', 'worker');
-- Modify "users" table
ALTER TABLE "public"."users" DROP CONSTRAINT "users_location_check", ADD CONSTRAINT "users_location_check" CHECK ((location IS NULL) OR (((length(TRIM(BOTH FROM location)) >= 5) AND (length(TRIM(BOTH FROM location)) <= 62)) AND (location ~ '^[A-Z]{2},\s[[:alpha:]\s\.-]+$'::text))), ADD COLUMN "role" "public"."user_role" NOT NULL DEFAULT 'user';
-- Create "session" table
CREATE TABLE "public"."session" (
  "user_id" uuid NOT NULL,
  "token" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "expires_at" timestamptz NOT NULL,
  CONSTRAINT "session_token_pkey" PRIMARY KEY ("token"),
  CONSTRAINT "session_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "session_user_id_idx" to table: "session"
CREATE INDEX "session_user_id_idx" ON "public"."session" ("user_id");

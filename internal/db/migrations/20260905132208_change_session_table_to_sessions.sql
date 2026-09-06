-- Create "sessions" table
CREATE TABLE "public"."sessions" (
  "user_id" uuid NOT NULL,
  "token" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "expires_at" timestamptz NOT NULL,
  CONSTRAINT "sessions_token_pkey" PRIMARY KEY ("token"),
  CONSTRAINT "sessions_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "sessions_user_id_idx" to table: "sessions"
CREATE INDEX "sessions_user_id_idx" ON "public"."sessions" ("user_id");
-- Drop "session" table
DROP TABLE "public"."session";

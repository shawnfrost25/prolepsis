-- Create "pending_registrations" table
CREATE TABLE "public"."pending_registrations" (
  "token" text NOT NULL,
  "name" text NOT NULL,
  "gender" text NOT NULL,
  "birth_date" date NOT NULL,
  "email" text NOT NULL,
  "password_hash" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "expires_at" timestamptz NOT NULL,
  CONSTRAINT "pending_registrations_token_pkey" PRIMARY KEY ("token"),
  CONSTRAINT "pending_registrations_email_check" CHECK (((length(TRIM(BOTH FROM email)) >= 6) AND (length(TRIM(BOTH FROM email)) <= 254)) AND (email ~* '^[a-z0-9._+-]{1,64}@([a-z0-9-]{1,63}\.)+([a-z0-9]{2,18})$'::text)),
  CONSTRAINT "pending_registrations_gender_check" CHECK (gender = ANY (ARRAY['male'::text, 'female'::text, 'prefer_not_to_specify'::text])),
  CONSTRAINT "pending_registrations_name_check" CHECK ((length(TRIM(BOTH FROM name)) >= 3) AND (length(TRIM(BOTH FROM name)) <= 25)),
  CONSTRAINT "pending_registrations_password_hash_check" CHECK (length(password_hash) >= 60)
);

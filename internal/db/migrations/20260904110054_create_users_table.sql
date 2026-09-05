-- Create "users" table
CREATE TABLE "public"."users" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "name" text NOT NULL,
  "display_name" text NULL,
  "bio" text NULL,
  "gender" text NOT NULL,
  "location" text NULL,
  "birth_date" date NOT NULL,
  "email" text NOT NULL,
  "password_hash" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT "users_id_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "users_email_key" UNIQUE ("email"),
  CONSTRAINT "users_name_key" UNIQUE ("name"),
  CONSTRAINT "users_bio_check" CHECK ((bio IS NULL) OR (length(TRIM(BOTH FROM bio)) <= 250)),
  CONSTRAINT "users_display_name_check" CHECK ((display_name IS NULL) OR ((length(TRIM(BOTH FROM display_name)) >= 1) AND (length(TRIM(BOTH FROM display_name)) <= 30))),
  CONSTRAINT "users_email_check" CHECK (((length(TRIM(BOTH FROM email)) >= 6) AND (length(TRIM(BOTH FROM email)) <= 254)) AND (email ~* '^[a-z0-9._+-]{1,64}@([a-z0-9-]{1,63}\.)+([a-z0-9]{2,18})$'::text)),
  CONSTRAINT "users_gender_check" CHECK (gender = ANY (ARRAY['male'::text, 'female'::text, 'prefer_not_to_specify'::text])),
  CONSTRAINT "users_location_check" CHECK ((location IS NULL) OR (((length(TRIM(BOTH FROM location)) >= 5) AND (length(TRIM(BOTH FROM location)) <= 62)) AND (location ~ '^[A-Z]{2},\s[A-Za-z\s\.-]+$'::text))),
  CONSTRAINT "users_name_check" CHECK ((length(TRIM(BOTH FROM name)) >= 3) AND (length(TRIM(BOTH FROM name)) <= 25)),
  CONSTRAINT "users_password_hash_check" CHECK (length(password_hash) >= 60)
);
-- Create "default_display_to_name" function
CREATE FUNCTION "public"."default_display_to_name" () RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    -- In easy words: If the value for 'display_name' is empty or NULL, we default it to 'name' 
    IF NEW.display_name IS NULL OR length(trim(NEW.display_name)) = 0 THEN
        NEW.display_name := NEW.name;
    END IF;
    RETURN NEW;
END;
$$;
-- Create trigger "trg_default_display_to_name"
CREATE TRIGGER "trg_default_display_to_name" BEFORE INSERT OR UPDATE ON "public"."users" FOR EACH ROW EXECUTE FUNCTION "public"."default_display_to_name"();

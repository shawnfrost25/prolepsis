-- Rename a column from "gender" to "sex"
ALTER TABLE "public"."pending_registrations" RENAME COLUMN "gender" TO "sex";
-- Modify "pending_registrations" table
ALTER TABLE "public"."pending_registrations" DROP CONSTRAINT "pending_registrations_gender_check", ADD CONSTRAINT "pending_registrations_sex_check" CHECK (sex = ANY (ARRAY['male'::text, 'female'::text, 'prefer_not_to_specify'::text]));
-- Rename a column from "gender" to "sex"
ALTER TABLE "public"."users" RENAME COLUMN "gender" TO "sex";
-- Modify "users" table
ALTER TABLE "public"."users" DROP CONSTRAINT "users_gender_check", ADD CONSTRAINT "users_sex_check" CHECK (sex = ANY (ARRAY['male'::text, 'female'::text, 'prefer_not_to_specify'::text]));

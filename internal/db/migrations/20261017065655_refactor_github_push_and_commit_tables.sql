-- Modify "github_repo" table
ALTER TABLE "public"."github_repo" ALTER COLUMN "created_at" DROP NOT NULL, ALTER COLUMN "total_releases" DROP NOT NULL, ALTER COLUMN "total_releases" SET DEFAULT 0, ADD COLUMN "action" text NULL, ADD COLUMN "updated_at" timestamptz NULL;
-- Rename a constraint from "github_license_license_key_pkey" to "github_repo_license_license_key_pkey"
ALTER TABLE "public"."github_repo_license" RENAME CONSTRAINT "github_license_license_key_pkey" TO "github_repo_license_license_key_pkey";
-- Modify "issue_requests" table
ALTER TABLE "public"."issue_requests" ALTER COLUMN "is_closed" SET NOT NULL, ALTER COLUMN "is_locked" SET NOT NULL, ADD COLUMN "is_transferred" boolean NULL DEFAULT false;
-- Modify "pull_requests" table
ALTER TABLE "public"."pull_requests" ALTER COLUMN "body" DROP NOT NULL, ALTER COLUMN "participants_number" DROP NOT NULL, ALTER COLUMN "positive_reactions" DROP NOT NULL, ALTER COLUMN "negative_reactions" DROP NOT NULL, ALTER COLUMN "total_reactions" DROP NOT NULL;
-- Modify "release_requests" table
ALTER TABLE "public"."release_requests" ALTER COLUMN "tag_name" SET NOT NULL;
-- Create "github_webhook_deliveries" table
CREATE TABLE "public"."github_webhook_deliveries" (
  "delivery_id" uuid NOT NULL,
  CONSTRAINT "github_webhook_deliveries_delivery_id_pkey" PRIMARY KEY ("delivery_id")
);
-- Rename the legacy "push_webhook" primary key before creating "push_requests"
ALTER TABLE "public"."push_webhook" RENAME CONSTRAINT "push_requests_id_pkey" TO "push_webhook_id_pkey";
-- Create "push_requests" table
CREATE TABLE "public"."push_requests" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "github_repo_id" bigint NOT NULL,
  "github_user_id" bigint NOT NULL,
  "ref" text NOT NULL,
  "before_sha" text NULL,
  "after_sha" text NULL,
  "is_forced" boolean NOT NULL DEFAULT false,
  "is_created" boolean NOT NULL DEFAULT false,
  "html_url" text NOT NULL,
  "pushed_at" timestamptz NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT "push_requests_id_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "push_requests_github_repo_id_fkey" FOREIGN KEY ("github_repo_id") REFERENCES "public"."github_repo" ("repo_id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "push_requests_github_user_id_fkey" FOREIGN KEY ("github_user_id") REFERENCES "public"."github_user" ("github_id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create "push_commits" table
CREATE TABLE "public"."push_commits" (
  "push_id" uuid NOT NULL,
  "commit_sha" text NOT NULL,
  "message" text NOT NULL,
  "added" text[] NULL DEFAULT '{}',
  "removed" text[] NULL DEFAULT '{}',
  "modified" text[] NULL DEFAULT '{}',
  "committed_at" timestamptz NOT NULL,
  CONSTRAINT "push_commits_commit_sha_pkey" PRIMARY KEY ("commit_sha"),
  CONSTRAINT "push_commits_push_id_fkey" FOREIGN KEY ("push_id") REFERENCES "public"."push_requests" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Drop "github_repo_webhook_deliveries" table
DROP TABLE "public"."github_repo_webhook_deliveries";
-- Drop "github_repo_push_commit" table
DROP TABLE "public"."github_repo_push_commit";
-- Drop "push_webhook" table
DROP TABLE "public"."push_webhook";

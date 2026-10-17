-- Rename a column from "url" to "html_url"
ALTER TABLE "public"."github_repo_license" RENAME COLUMN "url" TO "html_url";
-- Modify "github_repo_license" table
ALTER TABLE "public"."github_repo_license" ALTER COLUMN "spdx_id" DROP NOT NULL, ADD COLUMN "body" text NOT NULL;
-- Rename a column from "name" to "login"
ALTER TABLE "public"."github_user" RENAME COLUMN "name" TO "login";
-- Modify "github_user" table
ALTER TABLE "public"."github_user" DROP COLUMN "hireable", ALTER COLUMN "followers" SET NOT NULL, DROP COLUMN "total_public_repos", DROP COLUMN "total_private_repos", ALTER COLUMN "total_repos" TYPE integer, ADD COLUMN "status_message" text NULL, ADD COLUMN "is_hireable" boolean NOT NULL;
-- Modify "pending_deletions" table
ALTER TABLE "public"."pending_deletions" ALTER COLUMN "clarification" DROP NOT NULL;
-- Rename a column from "archived" to "is_archived"
ALTER TABLE "public"."github_repo" RENAME COLUMN "archived" TO "is_archived";
-- Modify "github_repo" table
ALTER TABLE "public"."github_repo" DROP CONSTRAINT "github_repo_github_user_id_fkey", ALTER COLUMN "pushed_at" DROP NOT NULL, ALTER COLUMN "stars" SET NOT NULL, ALTER COLUMN "watches" SET NOT NULL, ALTER COLUMN "forks_count" SET NOT NULL, DROP COLUMN "topic", ADD COLUMN "hook_id" bigint NULL, ADD COLUMN "readme" text NULL, ADD COLUMN "secondary_languages" text[] NULL DEFAULT '{}', ADD COLUMN "topics" text[] NULL DEFAULT '{}', ADD COLUMN "total_releases" integer NOT NULL, ADD CONSTRAINT "github_repo_hook_id_key" UNIQUE ("hook_id"), ADD CONSTRAINT "github_repo_github_id_fkey" FOREIGN KEY ("github_id") REFERENCES "public"."github_user" ("github_id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Create "push_webhook" table
CREATE TABLE "public"."push_webhook" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "github_repo_id" bigint NOT NULL,
  "github_user_id" bigint NOT NULL,
  "hook_id" bigint NOT NULL,
  "full_name" text NOT NULL,
  "ref" text NOT NULL,
  "before_sha" text NULL,
  "after_sha" text NULL,
  "head_commit_id" text NULL,
  "compare" text NOT NULL,
  "forced" boolean NOT NULL DEFAULT false,
  "created" boolean NOT NULL DEFAULT false,
  "deleted" boolean NOT NULL DEFAULT false,
  "pushed_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT "push_requests_id_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "github_repo_push_github_repo_id_fkey" FOREIGN KEY ("github_repo_id") REFERENCES "public"."github_repo" ("repo_id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "github_repo_push_github_user_id_fkey" FOREIGN KEY ("github_user_id") REFERENCES "public"."github_user" ("github_id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Modify "github_repo_push_commit" table
ALTER TABLE "public"."github_repo_push_commit" DROP CONSTRAINT "github_repo_push_commit_push_id_fkey", ADD CONSTRAINT "github_repo_push_commit_push_id_fkey" FOREIGN KEY ("push_id") REFERENCES "public"."push_webhook" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
-- Create "issue_requests" table
CREATE TABLE "public"."issue_requests" (
  "id" bigint NOT NULL,
  "github_repo_id" bigint NOT NULL,
  "github_user_id" bigint NOT NULL,
  "action" text NULL,
  "author_login" text NULL,
  "title" text NOT NULL,
  "body" text NOT NULL,
  "state" text NOT NULL,
  "state_reason" text NULL,
  "issue_type" text NULL,
  "labels" text[] NULL DEFAULT '{}',
  "number" integer NOT NULL,
  "participants_number" integer NOT NULL,
  "positive_reactions" integer NOT NULL,
  "negative_reactions" integer NOT NULL,
  "total_reactions" integer NOT NULL,
  "is_closed" boolean NULL,
  "is_locked" boolean NULL,
  "html_url" text NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "closed_at" timestamptz NULL,
  "last_edited_at" timestamptz NULL,
  CONSTRAINT "issue_requests_id_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "issue_requests_github_repo_id_fkey" FOREIGN KEY ("github_repo_id") REFERENCES "public"."github_repo" ("repo_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "issue_requests_github_user_id_fkey" FOREIGN KEY ("github_user_id") REFERENCES "public"."github_user" ("github_id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create "issue_comments" table
CREATE TABLE "public"."issue_comments" (
  "id" bigint NOT NULL,
  "issue_id" bigint NOT NULL,
  "action" text NULL,
  "author_login" text NULL,
  "body" text NOT NULL,
  "minimized_reason" text NULL,
  "is_minimized" boolean NOT NULL,
  "html_url" text NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "last_edited_at" timestamptz NULL,
  CONSTRAINT "issue_comments_id_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "issue_comments_issue_id_fkey" FOREIGN KEY ("issue_id") REFERENCES "public"."issue_requests" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create "pull_requests" table
CREATE TABLE "public"."pull_requests" (
  "id" bigint NOT NULL,
  "github_repo_id" bigint NOT NULL,
  "github_user_id" bigint NOT NULL,
  "action" text NULL,
  "author_login" text NULL,
  "title" text NOT NULL,
  "body" text NOT NULL,
  "state" text NOT NULL,
  "labels" text[] NULL DEFAULT '{}',
  "additions" integer NOT NULL,
  "changed_files" integer NOT NULL,
  "deletions" integer NOT NULL,
  "number" integer NOT NULL,
  "participants_number" integer NOT NULL,
  "positive_reactions" integer NOT NULL,
  "negative_reactions" integer NOT NULL,
  "total_reactions" integer NOT NULL,
  "is_closed" boolean NOT NULL,
  "is_draft" boolean NOT NULL,
  "is_locked" boolean NOT NULL,
  "is_merged" boolean NOT NULL,
  "html_url" text NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "closed_at" timestamptz NULL,
  "merged_at" timestamptz NULL,
  CONSTRAINT "pull_requests_id_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "pull_requests_github_repo_id_fkey" FOREIGN KEY ("github_repo_id") REFERENCES "public"."github_repo" ("repo_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "pull_requests_github_user_id_fkey" FOREIGN KEY ("github_user_id") REFERENCES "public"."github_user" ("github_id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create "pull_request_comments" table
CREATE TABLE "public"."pull_request_comments" (
  "id" bigint NOT NULL,
  "pr_id" bigint NOT NULL,
  "action" text NULL,
  "author_login" text NULL,
  "body" text NOT NULL,
  "minimized_reason" text NULL,
  "is_minimized" boolean NOT NULL,
  "html_url" text NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "last_edited_at" timestamptz NULL,
  CONSTRAINT "pull_request_comments_id_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "pull_request_comments_pr_id_fkey" FOREIGN KEY ("pr_id") REFERENCES "public"."pull_requests" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create "release_requests" table
CREATE TABLE "public"."release_requests" (
  "id" bigint NOT NULL,
  "github_repo_id" bigint NOT NULL,
  "github_user_id" bigint NOT NULL,
  "action" text NULL,
  "author_login" text NULL,
  "name" text NULL,
  "tag_name" text NULL,
  "description" text NULL,
  "is_draft" boolean NOT NULL,
  "is_prereleased" boolean NOT NULL,
  "is_immutable" boolean NOT NULL,
  "html_url" text NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "published_at" timestamptz NULL,
  CONSTRAINT "release_requests_id_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "release_requests_github_repo_id_fkey" FOREIGN KEY ("github_repo_id") REFERENCES "public"."github_repo" ("repo_id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "release_requests_github_user_id_fkey" FOREIGN KEY ("github_user_id") REFERENCES "public"."github_user" ("github_id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Drop "github_repo_push" table
DROP TABLE "public"."github_repo_push";
-- Drop "github_repo_releases" table
DROP TABLE "public"."github_repo_releases";
-- Drop "github_repo_issues_comments" table
DROP TABLE "public"."github_repo_issues_comments";
-- Drop "github_repo_issues_multi_select_options" table
DROP TABLE "public"."github_repo_issues_multi_select_options";
-- Drop "github_repo_issues_field_values" table
DROP TABLE "public"."github_repo_issues_field_values";
-- Drop "github_repo_issues_labels" table
DROP TABLE "public"."github_repo_issues_labels";
-- Drop "github_repo_issues_info" table
DROP TABLE "public"."github_repo_issues_info";
-- Drop "github_repo_issues" table
DROP TABLE "public"."github_repo_issues";
-- Drop "github_repo_pull_request" table
DROP TABLE "public"."github_repo_pull_request";
-- Drop "github_repo_pull_request_info" table
DROP TABLE "public"."github_repo_pull_request_info";

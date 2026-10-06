-- Create "github_repo_pull_request_info" table
CREATE TABLE "public"."github_repo_pull_request_info" (
  "id" bigint NOT NULL,
  "number" integer NOT NULL,
  "title" text NOT NULL,
  "state" text NOT NULL,
  "is_draft" boolean NOT NULL,
  "is_merged" boolean NULL,
  "author_id" bigint NULL,
  "author_name" text NULL,
  "author_type" text NULL,
  CONSTRAINT "github_repo_pull_request_info_id_pkey" PRIMARY KEY ("id")
);
-- Create "github_repo_pull_request" table
CREATE TABLE "public"."github_repo_pull_request" (
  "github_repo_id" bigint NOT NULL,
  "github_user_id" bigint NOT NULL,
  "hook_id" bigint NOT NULL,
  "event_action" text NOT NULL,
  "full_name" text NOT NULL,
  "sender_id" bigint NOT NULL,
  "sender_name" text NOT NULL,
  "sender_type" text NOT NULL,
  "pull_request_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "closed_at" timestamptz NULL,
  "merged_at" timestamptz NULL,
  "additions" integer NOT NULL,
  "deletions" integer NOT NULL,
  "changed_files" integer NOT NULL,
  "commits_count" integer NOT NULL,
  "comments_count" integer NOT NULL,
  "review_comments_count" integer NOT NULL,
  "head_branch" text NOT NULL,
  "head_sha" text NOT NULL,
  "base_branch" text NOT NULL,
  "merge_commit_sha" text NULL,
  "assignee_ids" bigint[] NULL DEFAULT '{}',
  "requested_reviewer_ids" bigint[] NULL DEFAULT '{}',
  "requested_team_ids" bigint[] NULL DEFAULT '{}',
  "labels" text[] NULL DEFAULT '{}',
  "milestone_id" bigint NULL,
  CONSTRAINT "github_repo_pull_request_github_repo_id_fkey" FOREIGN KEY ("github_repo_id") REFERENCES "public"."github_repo" ("repo_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "github_repo_pull_request_github_user_id_fkey" FOREIGN KEY ("github_user_id") REFERENCES "public"."github_user" ("github_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "github_repo_pull_request_pull_request_id_fkey" FOREIGN KEY ("pull_request_id") REFERENCES "public"."github_repo_pull_request_info" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Modify "github_repo_push" table
ALTER TABLE "public"."github_repo_push" DROP CONSTRAINT "github_repo_push_github_user_id", ADD CONSTRAINT "github_repo_push_github_user_id_fkey" FOREIGN KEY ("github_user_id") REFERENCES "public"."github_user" ("github_id") ON UPDATE NO ACTION ON DELETE CASCADE;

-- Modify "github_repo_issues" table
ALTER TABLE "public"."github_repo_issues" ADD COLUMN "sender_id" bigint NOT NULL, ADD COLUMN "sender_name" text NOT NULL, ADD COLUMN "sender_type" text NOT NULL;
-- Modify "github_repo_issues_info" table
ALTER TABLE "public"."github_repo_issues_info" ADD COLUMN "assignee_id" bigint NULL, ADD COLUMN "assignee_name" text NULL, ADD COLUMN "assignee_type" text NULL;
-- Create "github_repo_webhook_deliveries" table
CREATE TABLE "public"."github_repo_webhook_deliveries" (
  "delivery_id" uuid NOT NULL,
  CONSTRAINT "github_repo_webhook_deliveries_delivery_id_pkey" PRIMARY KEY ("delivery_id")
);
-- Create "github_repo_issues_comments" table
CREATE TABLE "public"."github_repo_issues_comments" (
  "issue_id" bigint NOT NULL,
  "action" text NOT NULL,
  "change_from" text NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "author_association" text NOT NULL,
  "comment_id" bigint NOT NULL,
  "commentor_id" bigint NOT NULL,
  "commentor_name" text NOT NULL,
  "commentor_type" text NOT NULL,
  "body" text NOT NULL,
  "positive_reactions" integer NOT NULL,
  "negative_reactions" integer NOT NULL,
  "total_reactions" integer NOT NULL,
  CONSTRAINT "github_repo_issues_comments_issue_id_fkey" FOREIGN KEY ("issue_id") REFERENCES "public"."github_repo_issues_info" ("issue_id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create "github_repo_releases" table
CREATE TABLE "public"."github_repo_releases" (
  "github_repo_id" bigint NOT NULL,
  "github_user_id" bigint NOT NULL,
  "releases_id" bigint NOT NULL,
  "hook_id" bigint NOT NULL,
  "action" text NOT NULL,
  "tag_name" text NOT NULL,
  "name" text NOT NULL,
  "body" text NOT NULL,
  "target_commitish" text NOT NULL,
  "draft" boolean NOT NULL,
  "prerelease" boolean NOT NULL,
  "published_at" timestamptz NULL,
  "sender_id" bigint NOT NULL,
  "sender_name" text NOT NULL,
  "sender_type" text NOT NULL,
  CONSTRAINT "github_repo_releases_releases_id_pkey" PRIMARY KEY ("releases_id"),
  CONSTRAINT "github_repo_releases_github_repo_id_fkey" FOREIGN KEY ("github_repo_id") REFERENCES "public"."github_repo" ("repo_id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "github_repo_releases_github_user_id_fkey" FOREIGN KEY ("github_user_id") REFERENCES "public"."github_user" ("github_id") ON UPDATE NO ACTION ON DELETE CASCADE
);

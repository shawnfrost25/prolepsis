-- Create "github_repo_issues" table
CREATE TABLE "public"."github_repo_issues" (
  "id" uuid NOT NULL,
  "github_repo_id" bigint NOT NULL,
  "github_user_id" bigint NOT NULL,
  "hook_id" bigint NOT NULL,
  "event_action" text NOT NULL,
  "assignee_id" bigint NULL,
  "assignee_name" text NULL,
  "assignee_type" text NULL,
  CONSTRAINT "github_repo_issues_id_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "github_repo_issues_github_repo_id_fkey" FOREIGN KEY ("github_repo_id") REFERENCES "public"."github_repo" ("repo_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "github_repo_issues_github_user_id_fkey" FOREIGN KEY ("github_user_id") REFERENCES "public"."github_user" ("github_id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create "github_repo_issues_info" table
CREATE TABLE "public"."github_repo_issues_info" (
  "related_id" uuid NOT NULL,
  "issue_id" bigint NOT NULL,
  "author_association" text NOT NULL,
  "body" text NULL,
  "comments" integer NOT NULL,
  "draft" boolean NOT NULL,
  "created_at" timestamptz NOT NULL,
  "deleted_at" timestamptz NULL,
  "locked" boolean NOT NULL,
  "milestone_description" text NULL,
  "milestone_due_on" text NULL,
  "milestone_state" text NULL,
  "milestone_title" text NULL,
  "number" integer NOT NULL,
  "positive_reactions" integer NOT NULL,
  "negative_reactions" integer NOT NULL,
  "reactions_total_count" integer NOT NULL,
  "state" text NOT NULL,
  "state_reason" text NULL,
  "sub_issue_total" integer NOT NULL,
  "sub_issue_completed" integer NOT NULL,
  "sub_issue_percent_completed" integer NOT NULL,
  "issue_dependency_total_blocked_by" integer NOT NULL,
  "issue_dependency_total_blocking" integer NOT NULL,
  "title" text NOT NULL,
  "type_name" text NULL,
  "type_description" text NULL,
  "updated_at" timestamptz NOT NULL,
  "issue_created_by" bigint NULL,
  "creator_name" text NULL,
  "creator_type" text NULL,
  PRIMARY KEY ("issue_id"),
  CONSTRAINT "github_repo_issues_info_related_id_key" UNIQUE ("related_id"),
  CONSTRAINT "github_repo_issues_info_related_id_fkey" FOREIGN KEY ("related_id") REFERENCES "public"."github_repo_issues" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create "github_repo_issues_field_values" table
CREATE TABLE "public"."github_repo_issues_field_values" (
  "id" uuid NOT NULL,
  "issue_id" bigint NOT NULL,
  "issue_field_name" text NOT NULL,
  "data_type" text NOT NULL,
  "value" jsonb NULL,
  "single_select_option_id" bigint NULL,
  "single_select_option_name" text NULL,
  CONSTRAINT "github_repo_issues_field_values_id_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "github_repo_issues_field_values_issue_id_fkey" FOREIGN KEY ("issue_id") REFERENCES "public"."github_repo_issues_info" ("issue_id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create "github_repo_issues_labels" table
CREATE TABLE "public"."github_repo_issues_labels" (
  "issue_id" bigint NOT NULL,
  "label_id" bigint NOT NULL,
  "name" text NOT NULL,
  "description" text NULL,
  CONSTRAINT "github_repo_issues_labels_label_issue_key" UNIQUE ("issue_id", "label_id"),
  CONSTRAINT "github_repo_issues_label_issue_id_fkey" FOREIGN KEY ("issue_id") REFERENCES "public"."github_repo_issues_info" ("issue_id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create "github_repo_issues_multi_select_options" table
CREATE TABLE "public"."github_repo_issues_multi_select_options" (
  "field_value_id" uuid NOT NULL,
  "id" bigint NOT NULL,
  "name" text NOT NULL,
  CONSTRAINT "mgithub_repo_issues_multi_select_options_id_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "github_repo_issues_multi_select_options_field_value_id_fkey" FOREIGN KEY ("field_value_id") REFERENCES "public"."github_repo_issues_field_values" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);

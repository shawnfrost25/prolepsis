-- Create "github_repo_push" table
CREATE TABLE "public"."github_repo_push" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "github_repo_id" bigint NOT NULL,
  "github_user_id" bigint NOT NULL,
  "full_name" text NOT NULL,
  "ref" text NOT NULL,
  "before_sha" text NOT NULL,
  "after_sha" text NOT NULL,
  "head_commit_id" text NOT NULL,
  "compare" text NOT NULL,
  "forced" boolean NOT NULL DEFAULT false,
  "created" boolean NOT NULL DEFAULT false,
  "deleted" boolean NOT NULL DEFAULT false,
  "pushed_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT "github_repo_push_id" PRIMARY KEY ("id"),
  CONSTRAINT "github_repo_webhook_github_repo_id_fkey" FOREIGN KEY ("github_repo_id") REFERENCES "public"."github_repo" ("repo_id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "github_repo_webhook_github_user_id" FOREIGN KEY ("github_user_id") REFERENCES "public"."github_user" ("github_id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create "github_repo_push_commit" table
CREATE TABLE "public"."github_repo_push_commit" (
  "push_id" uuid NOT NULL,
  "commit_sha" text NOT NULL,
  "message" text NULL DEFAULT 'NONE',
  "added" text[] NULL DEFAULT '{}',
  "removed" text[] NULL DEFAULT '{}',
  "modified" text[] NULL DEFAULT '{}',
  "committed_at" timestamptz NOT NULL,
  CONSTRAINT "github_repo_push_commit_commit_sha_pkey" PRIMARY KEY ("commit_sha"),
  CONSTRAINT "github_repo_push_commit_push_id_fkey" FOREIGN KEY ("push_id") REFERENCES "public"."github_repo_push" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);

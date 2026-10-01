-- Modify "github_repo_push" table
ALTER TABLE "public"."github_repo_push" ALTER COLUMN "before_sha" DROP NOT NULL, ALTER COLUMN "after_sha" DROP NOT NULL, ALTER COLUMN "head_commit_id" DROP NOT NULL, ADD COLUMN "hook_id" bigint NOT NULL;
-- Modify "github_repo_push_commit" table
ALTER TABLE "public"."github_repo_push_commit" ALTER COLUMN "message" SET NOT NULL, ALTER COLUMN "message" DROP DEFAULT, ADD COLUMN "url" text NOT NULL;

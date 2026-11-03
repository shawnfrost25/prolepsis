-- Modify "issue_comments" table
ALTER TABLE "public"."issue_comments" ALTER COLUMN "is_minimized" DROP NOT NULL, DROP COLUMN "last_edited_at";
-- Modify "issue_requests" table
ALTER TABLE "public"."issue_requests" DROP COLUMN "last_edited_at";
-- Modify "pull_request_comments" table
ALTER TABLE "public"."pull_request_comments" DROP COLUMN "last_edited_at";
-- Modify "push_requests" table
ALTER TABLE "public"."push_requests" DROP COLUMN "html_url";

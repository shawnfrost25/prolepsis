-- We use this to insert data about the user inside the 'github_user' table
-- name: InsertGitHubUserInfo :exec
INSERT INTO github_user (user_id, github_id, name, avatar_url, html_url, company, email, hireable, bio, followers, total_public_repos, total_private_repos, total_repos, created_at)
VALUES(
    sqlc.arg('user_id')::uuid, sqlc.arg('github_id')::bigint, sqlc.arg('name')::text, sqlc.arg('avatar_url')::text, sqlc.arg('html_url')::text, sqlc.narg('company')::text, sqlc.narg('email')::text, sqlc.narg('hireable')::boolean, sqlc.narg('bio')::text, sqlc.narg('followers')::int, sqlc.arg('total_public_repos')::smallint, sqlc.arg('total_private_repos')::smallint, sqlc.arg('total_repos')::smallint, sqlc.arg('created_at')
) ON CONFLICT (github_id) DO UPDATE SET
name = EXCLUDED.name,
company = EXCLUDED.company,
email = EXCLUDED.email,
hireable = EXCLUDED.hireable,
bio = EXCLUDED.bio,
followers = EXCLUDED.followers,
total_public_repos = EXCLUDED.total_public_repos,
total_private_repos = EXCLUDED.total_private_repos,
total_repos = EXCLUDED.total_repos;

-- We use this to check if the oauth2 is already done (there is already a value)
-- name: UserExistsInsideGitHub :one
SELECT EXISTS (
    SELECT 1 FROM github_user WHERE user_id = $1
);

-- We insert data inside the 'github_repo' table
-- name: InsertGitHubRepoInfo :exec
INSERT INTO github_repo (github_id, repo_id, name, html_url, description, created_at, pushed_at, stars, watches, forks_count, main_language, license_key, topic, archived, visibility)
VALUES(
    sqlc.arg('github_id')::bigint, sqlc.arg('repo_id')::bigint, sqlc.arg('name')::text, sqlc.arg('html_url')::text, sqlc.narg('description')::text, sqlc.arg('created_at')::timestamptz, sqlc.arg('pushed_at')::timestamptz, sqlc.narg('stars')::int, sqlc.narg('watches')::int, sqlc.narg('forks_count')::int, sqlc.narg('main_language')::text, sqlc.narg('license_key')::text, sqlc.arg('topic')::text[], sqlc.arg('archived')::boolean, sqlc.arg('visibility')::text
) ON CONFLICT (repo_id) DO UPDATE SET
name = EXCLUDED.name,
description = EXCLUDED.description,
pushed_at = EXCLUDED.pushed_at,
stars = EXCLUDED.stars,
watches = EXCLUDED.watches,
forks_count = EXCLUDED.forks_count,
main_language = EXCLUDED.main_language,
license_key = EXCLUDED.license_key,
topic = EXCLUDED.topic,
archived = EXCLUDED.archived,
visibility = EXCLUDED.visibility;

-- We use this to check if there is already the same repository
-- name: RepoExistsInsideGitHub :one
SELECT EXISTS (
    SELECT 1 FROM github_repo WHERE repo_id = $1
);

-- name: InsertGitHubLicenseInfo :exec
INSERT INTO github_repo_license(license_key, name, spdx_id, url, node_id)
VALUES(
    sqlc.arg('license_key')::text, sqlc.arg('name')::text, sqlc.arg('spdx_id')::text, sqlc.narg('url')::text, sqlc.arg('node_id')::text
);

-- name: LicenseExistsInsideGitHub :one
SELECT EXISTS (
    SELECT 1 FROM github_repo_license WHERE license_key = $1
);

-- name: InsertGitHubPushInfo :one
INSERT INTO github_repo_push(github_repo_id, github_user_id, hook_id, full_name, ref, before_sha, after_sha, head_commit_id, compare, forced, created, deleted)
VALUES (
    sqlc.arg('github_repo_id')::bigint, sqlc.arg('github_user_id')::bigint, sqlc.arg('hook_id')::bigint, sqlc.arg('full_name')::text, sqlc.arg('ref')::text, sqlc.narg('before_sha')::text, sqlc.narg('after_sha')::text, sqlc.narg('head_commit_id')::text, sqlc.arg('compare')::text, sqlc.arg('forced')::boolean, sqlc.arg('created')::boolean, sqlc.arg('deleted')::boolean
)
RETURNING id;

-- name: InsertGitHubPushCommitInfo :exec
INSERT INTO github_repo_push_commit(push_id, commit_sha, message, added, removed, modified, url, committed_at)
VALUES(
    sqlc.arg('push_id')::uuid, sqlc.arg('commit_sha')::text, sqlc.arg('message')::text, sqlc.arg('added')::text[], sqlc.arg('removed')::text[], sqlc.arg('modified')::text[], sqlc.arg('url')::text, sqlc.arg('committed_at')::timestamptz
);

-- name: DeleteGitHubMock :exec
TRUNCATE TABLE github_user, github_repo, github_repo_license, github_repo_push, github_repo_push_commit CASCADE;

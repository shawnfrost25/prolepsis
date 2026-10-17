-- INSERT (EVERYTHING BELOW) --

-- name: InsertGitHubUser :exec
INSERT INTO github_user(
    user_id,
    github_id,
    login,
    company,
    email,
    bio,
    status_message,
    followers,
    total_repos,
    is_hireable,
    avatar_url,
    html_url,
    created_at
) VALUES (
    sqlc.arg('user_id')::uuid,
    sqlc.arg('github_id')::bigint,
    sqlc.arg('login')::text,
    sqlc.narg('company')::text,
    sqlc.narg('email')::text,
    sqlc.narg('bio')::text,
    sqlc.narg('status_message')::text,
    sqlc.arg('followers')::int,
    sqlc.arg('total_repos')::int,
    sqlc.arg('is_hireable')::boolean,
    sqlc.arg('avatar_url')::text,
    sqlc.arg('html_url')::text,
    sqlc.arg('created_at')::timestamptz
) ON CONFLICT (github_id) DO UPDATE SET
    login = EXCLUDED.login,
    company = EXCLUDED.company,
    email = EXCLUDED.email,
    bio = EXCLUDED.bio,
    status_message = EXCLUDED.status_message,
    followers = EXCLUDED.followers,
    total_repos = EXCLUDED.total_repos,
    is_hireable = EXCLUDED.is_hireable,
    avatar_url = EXCLUDED.avatar_url,
    html_url = EXCLUDED.html_url;

-- name: InsertGitHubRepo :exec
INSERT INTO github_repo (
    github_id,
    repo_id,
    hook_id,
    action,
    name,
    description,
    readme,
    main_language,
    secondary_languages,
    license_key,
    topics,
    visibility,
    stars,
    watches,
    forks_count,
    total_releases,
    is_archived,
    html_url,
    created_at,
    pushed_at,
    updated_at
) VALUES (
    sqlc.arg('github_id')::bigint,
    sqlc.arg('repo_id')::bigint,
    sqlc.arg('hook_id')::bigint,
    sqlc.narg('action')::text,
    sqlc.arg('name')::text,
    sqlc.narg('description')::text,
    sqlc.narg('readme')::text,
    sqlc.narg('main_language')::text,
    sqlc.narg('secondary_languages')::text[],
    sqlc.narg('license_key')::text,
    sqlc.narg('topics')::text[],
    sqlc.arg('visibility')::text,
    sqlc.arg('stars')::int,
    sqlc.arg('watches')::int,
    sqlc.arg('forks_count')::int,
    sqlc.arg('total_releases')::int,
    sqlc.arg('is_archived')::boolean,
    sqlc.arg('html_url')::text,
    COALESCE(sqlc.narg('created_at')::timestamptz, CURRENT_TIMESTAMP),
    sqlc.narg('pushed_at')::timestamptz,
    sqlc.narg('updated_at')::timestamptz
) ON CONFLICT (repo_id) DO UPDATE SET
    action = EXCLUDED.action,
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    readme = EXCLUDED.readme,
    main_language = EXCLUDED.main_language,
    secondary_languages = EXCLUDED.secondary_languages,
    license_key = EXCLUDED.license_key,
    topics = EXCLUDED.topics,
    visibility = EXCLUDED.visibility,
    stars = EXCLUDED.stars,
    watches = EXCLUDED.watches,
    forks_count = EXCLUDED.forks_count,
    total_releases = EXCLUDED.total_releases,
    is_archived = EXCLUDED.is_archived,
    html_url = EXCLUDED.html_url,
    pushed_at = EXCLUDED.pushed_at,
    updated_at = EXCLUDED.updated_at;

-- name: InsertGitHubLicense :exec
INSERT INTO github_repo_license(
    spdx_id,
    node_id,
    license_key,
    name,
    body,
    html_url
) VALUES (
    sqlc.narg('spdx_id')::text,
    sqlc.arg('node_id')::text,
    sqlc.arg('license_key')::text,
    sqlc.arg('name')::text,
    sqlc.arg('body')::text,
    sqlc.narg('html_url')
) ON CONFLICT (license_key) DO NOTHING;

-- name: InsertPullRequest :exec
INSERT INTO pull_requests (
    id,
    github_repo_id,
    github_user_id,
    action,
    author_login,
    title,
    body,
    state,
    labels,
    additions,
    changed_files,
    deletions,
    number,
    participants_number,
    positive_reactions,
    negative_reactions,
    total_reactions,
    is_closed,
    is_draft,
    is_locked,
    is_merged,
    html_url,
    created_at,
    updated_at,
    closed_at,
    merged_at
) VALUES (
    sqlc.arg('id')::bigint,
    sqlc.arg('github_repo_id')::bigint,
    sqlc.arg('github_user_id')::bigint,
    sqlc.narg('action')::text,
    sqlc.narg('author_login')::text,
    sqlc.arg('title')::text,
    sqlc.narg('body')::text,
    sqlc.arg('state')::text,
    sqlc.narg('labels')::text[],
    sqlc.arg('additions')::int,
    sqlc.arg('changed_files')::int,
    sqlc.arg('deletions')::int,
    sqlc.arg('number')::int,
    sqlc.narg('participants_number')::int,
    sqlc.narg('positive_reactions')::int,
    sqlc.narg('negative_reactions')::int,
    sqlc.narg('total_reactions')::int,
    sqlc.arg('is_closed')::boolean,
    sqlc.arg('is_draft')::boolean,
    sqlc.arg('is_locked')::boolean,
    sqlc.arg('is_merged')::boolean,
    sqlc.arg('html_url')::text,
    sqlc.arg('created_at')::timestamptz,
    sqlc.arg('updated_at')::timestamptz,
    sqlc.narg('closed_at')::timestamptz,
    sqlc.narg('merged_at')::timestamptz
) ON CONFLICT (id) DO UPDATE SET
    action = EXCLUDED.action,
    author_login = EXCLUDED.author_login,
    title = EXCLUDED.title,
    body = EXCLUDED.body,
    state = EXCLUDED.state,
    labels = EXCLUDED.labels,
    additions = EXCLUDED.additions,
    changed_files = EXCLUDED.changed_files,
    deletions = EXCLUDED.deletions,
    number = EXCLUDED.number,
    participants_number = EXCLUDED.participants_number,
    positive_reactions = EXCLUDED.positive_reactions,
    negative_reactions = EXCLUDED.negative_reactions,
    total_reactions = EXCLUDED.total_reactions,
    is_closed = EXCLUDED.is_closed,
    is_draft = EXCLUDED.is_draft,
    is_locked = EXCLUDED.is_locked,
    is_merged = EXCLUDED.is_merged,
    html_url = EXCLUDED.html_url,
    updated_at = EXCLUDED.updated_at,
    closed_at = EXCLUDED.closed_at,
    merged_at = EXCLUDED.merged_at;

-- name: InsertPullRequestComment :exec
INSERT INTO pull_request_comments (
    id,
    pr_id,
    action,
    author_login,
    body,
    minimized_reason,
    is_minimized,
    html_url,
    created_at,
    updated_at
) VALUES (
    sqlc.arg('id')::bigint,
    sqlc.arg('pr_id')::bigint,
    sqlc.narg('action')::text,
    sqlc.narg('author_login')::text,
    sqlc.arg('body')::text,
    sqlc.narg('minimized_reason')::text,
    sqlc.arg('is_minimized')::boolean,
    sqlc.arg('html_url')::text,
    sqlc.arg('created_at')::timestamptz,
    sqlc.arg('updated_at')::timestamptz
) ON CONFLICT (id) DO UPDATE SET
    action = EXCLUDED.action,
    author_login = EXCLUDED.author_login,
    body = EXCLUDED.body,
    minimized_reason = EXCLUDED.minimized_reason,
    is_minimized = EXCLUDED.is_minimized,
    html_url = EXCLUDED.html_url,
    updated_at = EXCLUDED.updated_at;

-- name: InsertIssue :exec
INSERT INTO issue_requests (
    id,
    github_repo_id,
    github_user_id,
    action,
    author_login,
    title,
    body,
    state,
    state_reason,
    issue_type,
    labels,
    number,
    participants_number,
    positive_reactions,
    negative_reactions,
    total_reactions,
    is_closed,
    is_locked,
    html_url,
    created_at,
    updated_at,
    closed_at
) VALUES (
    sqlc.arg('id')::bigint,
    sqlc.arg('github_repo_id')::bigint,
    sqlc.arg('github_user_id')::bigint,
    sqlc.narg('action')::text,
    sqlc.narg('author_login')::text,
    sqlc.arg('title')::text,
    sqlc.arg('body')::text,
    sqlc.arg('state')::text,
    sqlc.narg('state_reason')::text,
    sqlc.narg('issue_type')::text,
    sqlc.narg('labels')::text[],
    sqlc.arg('number')::int,
    sqlc.arg('participants_number')::int,
    sqlc.arg('positive_reactions')::int,
    sqlc.arg('negative_reactions')::int,
    sqlc.arg('total_reactions')::int,
    sqlc.arg('is_closed')::boolean,
    sqlc.arg('is_locked')::boolean,
    sqlc.arg('html_url')::text,
    sqlc.arg('created_at')::timestamptz,
    sqlc.arg('updated_at')::timestamptz,
    sqlc.narg('closed_at')::timestamptz
) ON CONFLICT (id) DO UPDATE SET
    action = EXCLUDED.action,
    author_login = EXCLUDED.author_login,
    title = EXCLUDED.title,
    body = EXCLUDED.body,
    state = EXCLUDED.state,
    state_reason = EXCLUDED.state_reason,
    issue_type = EXCLUDED.issue_type,
    labels = EXCLUDED.labels,
    number = EXCLUDED.number,
    participants_number = EXCLUDED.participants_number,
    positive_reactions = EXCLUDED.positive_reactions,
    negative_reactions = EXCLUDED.negative_reactions,
    total_reactions = EXCLUDED.total_reactions,
    is_closed = EXCLUDED.is_closed,
    is_locked = EXCLUDED.is_locked,
    html_url = EXCLUDED.html_url,
    updated_at = EXCLUDED.updated_at,
    closed_at = EXCLUDED.closed_at;

-- name: InsertIssueComment :exec
INSERT INTO issue_comments (
    id,
    issue_id,
    action,
    author_login,
    body,
    minimized_reason,
    is_minimized,
    html_url,
    created_at,
    updated_at
) VALUES (
    sqlc.arg('id')::bigint,
    sqlc.arg('issue_id')::bigint,
    sqlc.narg('action')::text,
    sqlc.narg('author_login')::text,
    sqlc.arg('body')::text,
    sqlc.narg('minimized_reason')::text,
    sqlc.arg('is_minimized')::boolean,
    sqlc.arg('html_url')::text,
    sqlc.arg('created_at')::timestamptz,
    sqlc.arg('updated_at')::timestamptz
) ON CONFLICT (id) DO UPDATE SET
    action = EXCLUDED.action,
    author_login = EXCLUDED.author_login,
    body = EXCLUDED.body,
    minimized_reason = EXCLUDED.minimized_reason,
    is_minimized = EXCLUDED.is_minimized,
    html_url = EXCLUDED.html_url,
    updated_at = EXCLUDED.updated_at;

-- name: InsertRelease :exec
INSERT INTO release_requests (
    id,
    github_repo_id,
    github_user_id,
    action,
    author_login,
    name,
    tag_name,
    description,
    is_draft,
    is_prereleased,
    is_immutable,
    html_url,
    created_at,
    updated_at,
    published_at
) VALUES (
    sqlc.arg('id')::bigint,
    sqlc.arg('github_repo_id')::bigint,
    sqlc.arg('github_user_id')::bigint,
    sqlc.narg('action')::text,
    sqlc.narg('author_login')::text,
    sqlc.narg('name')::text,
    sqlc.arg('tag_name')::text,
    sqlc.narg('description')::text,
    sqlc.arg('is_draft')::boolean,
    sqlc.arg('is_prereleased')::boolean,
    sqlc.arg('is_immutable')::boolean,
    sqlc.arg('html_url')::text,
    sqlc.arg('created_at')::timestamptz,
    sqlc.arg('updated_at')::timestamptz,
    sqlc.narg('published_at')::timestamptz
) ON CONFLICT (id) DO UPDATE SET
    action = EXCLUDED.action,
    author_login = EXCLUDED.author_login,
    name = EXCLUDED.name,
    tag_name = EXCLUDED.tag_name,
    description = EXCLUDED.description,
    is_draft = EXCLUDED.is_draft,
    is_prereleased = EXCLUDED.is_prereleased,
    is_immutable = EXCLUDED.is_immutable,
    html_url = EXCLUDED.html_url,
    updated_at = EXCLUDED.updated_at,
    published_at = EXCLUDED.published_at;

-- name: InsertWebhookDeliveries :execrows
INSERT INTO github_webhook_deliveries (
    delivery_id
)
VALUES(
    sqlc.arg('delivery_id')::uuid
) ON CONFLICT (delivery_id) DO NOTHING;

-- name: InsertPush :one
INSERT INTO push_requests (
    github_repo_id,
    github_user_id,
    ref,
    before_sha,
    after_sha,
    is_forced,
    is_created,
    html_url
) VALUES (
    sqlc.arg('github_repo_id')::bigint,
    sqlc.arg('github_user_id')::bigint,
    sqlc.arg('ref')::text,
    sqlc.narg('before_sha')::text,
    sqlc.narg('after_sha')::text,
    sqlc.arg('is_forced')::boolean,
    sqlc.arg('is_created')::boolean,
    sqlc.arg('html_url')::text
) RETURNING id;

-- name: InsertPushCommit :exec
INSERT INTO push_commits (
    push_id,
    commit_sha,
    message,
    added,
    removed,
    modified,
    committed_at
) VALUES (
    sqlc.arg('push_id')::uuid,
    sqlc.arg('commit_sha')::text,
    sqlc.arg('message')::text,
    sqlc.arg('added')::text[],
    sqlc.arg('removed')::text[],
    sqlc.arg('modified')::text[],
    sqlc.arg('committed_at')::timestamptz
) ON CONFLICT (commit_sha) DO NOTHING;

-- EXISTENCE CHECK (EVERYTHING BELOW) --

-- name: ExistGitHubRepo :one
SELECT EXISTS (
    SELECT 1 FROM github_repo WHERE repo_id = sqlc.arg('repo_id')::bigint
);

-- name: ExistIssue :one
SELECT EXISTS (
    SELECT 1 FROM issue_requests WHERE id = sqlc.arg('id')::bigint
);

-- name: ExistPullRequest :one
SELECT EXISTS (
    SELECT 1 FROM pull_requests WHERE id = sqlc.arg('id')::bigint
);

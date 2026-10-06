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
) RETURNING id;

-- name: InsertGitHubPushCommitInfo :exec
INSERT INTO github_repo_push_commit(push_id, commit_sha, message, added, removed, modified, url, committed_at)
VALUES(
    sqlc.arg('push_id')::uuid, sqlc.arg('commit_sha')::text, sqlc.arg('message')::text, sqlc.arg('added')::text[], sqlc.arg('removed')::text[], sqlc.arg('modified')::text[], sqlc.arg('url')::text, sqlc.arg('committed_at')::timestamptz
);

-- name: InsertGitHubPullRequestInfo :exec
INSERT INTO github_repo_pull_request_info(id, number, title, state, is_draft, is_merged, author_id, author_name, author_type)
VALUES(
    sqlc.arg('id')::bigint, sqlc.arg('number')::int, sqlc.arg('title')::text, sqlc.arg('state')::text, sqlc.arg('is_draft')::boolean, sqlc.narg('is_merged')::boolean, sqlc.narg('author_id')::bigint, sqlc.narg('author_name')::text, sqlc.narg('author_type')::text
);

-- name: InsertGitHubPullRequest :exec
INSERT INTO github_repo_pull_request(github_repo_id, github_user_id, hook_id, event_action, full_name, sender_id, sender_name, sender_type, pull_request_id, created_at, updated_at, closed_at, merged_at, additions, deletions, changed_files, commits_count, comments_count, review_comments_count, head_branch, head_sha, base_branch, merge_commit_sha, assignee_ids, requested_reviewer_ids, requested_team_ids, labels, milestone_id)
VALUES(
    sqlc.arg('github_repo_id')::bigint, sqlc.arg('github_user_id')::bigint, sqlc.arg('hook_id')::bigint, sqlc.arg('event_action')::text, sqlc.arg('full_name')::text, sqlc.arg('sender_id')::bigint, sqlc.arg('sender_name')::text, sqlc.arg('sender_type')::text, sqlc.arg('pull_request_id')::bigint, sqlc.arg('created_at')::timestamptz, sqlc.arg('updated_at')::timestamptz, sqlc.narg('closed_at')::timestamptz, sqlc.narg('merged_at')::timestamptz, sqlc.arg('additions')::int, sqlc.arg('deletions')::int, sqlc.arg('changed_files')::int, sqlc.arg('commits_count')::int, sqlc.arg('comments_count')::int, sqlc.arg('review_comments_count')::int, sqlc.arg('head_branch')::text, sqlc.arg('head_sha')::text, sqlc.arg('base_branch')::text, sqlc.narg('merge_commit_sha')::text, sqlc.arg('assignee_ids')::bigint[], sqlc.arg('requested_reviewer_ids')::bigint[], sqlc.arg('requested_team_ids')::bigint[], sqlc.arg('labels')::text[], sqlc.narg('milestone_id')::bigint
);

-- name: InsertGitHubIssueLabel :exec
INSERT INTO github_repo_issues_labels(issue_id, label_id, name, description)
VALUES(
    sqlc.arg('issue_id')::bigint, sqlc.arg('label_id')::bigint, sqlc.arg('name')::text, sqlc.narg('description')::text
) ON CONFLICT (issue_id, label_id) DO UPDATE SET
name = EXCLUDED.name,
description = EXCLUDED.description;

-- name: InsertGitHubIssueFieldValue :one
INSERT INTO github_repo_issues_field_values(issue_id, issue_field_name, data_type, value, single_select_option_id, single_select_option_name)
VALUES(
    sqlc.arg('issue_id')::bigint, sqlc.arg('issue_field_name')::text, sqlc.arg('data_type')::text, sqlc.narg('value')::jsonb, sqlc.narg('single_select_option_id')::bigint, sqlc.narg('single_select_option_name')::text
) RETURNING id;

-- name: InsertGitHubIssueMultiSelectOption :exec
INSERT INTO github_repo_issues_multi_select_options(field_value_id, id, name)
VALUES(
    sqlc.arg('field_value_id')::uuid, sqlc.arg('id')::bigint, sqlc.arg('name')::text
);

-- name: InsertGitHubIssueInfo :execrows
INSERT INTO github_repo_issues_info(related_id, issue_id, author_association, body, comments, draft, created_at, deleted_at, locked, milestone_description, milestone_due_on, milestone_state, milestone_title, number, positive_reactions, negative_reactions, reactions_total_count, state, state_reason, sub_issue_total, sub_issue_completed, sub_issue_percent_completed, issue_dependency_total_blocked_by, issue_dependency_total_blocking, title, type_name, type_description, updated_at, issue_created_by, creator_name, creator_type, assignee_id, assignee_name, assignee_type)
VALUES(
    sqlc.arg('related_id')::uuid, sqlc.arg('issue_id')::bigint, sqlc.arg('author_association')::text, sqlc.narg('body')::text, sqlc.arg('comments')::int, sqlc.arg('draft')::boolean, sqlc.arg('created_at')::timestamptz, sqlc.narg('deleted_at')::timestamptz, sqlc.arg('locked')::boolean, sqlc.narg('milestone_description')::text, sqlc.narg('milestone_due_on')::text, sqlc.narg('milestone_state')::text, sqlc.narg('milestone_title')::text, sqlc.arg('number')::int, sqlc.arg('positive_reactions')::int, sqlc.arg('negative_reactions')::int, sqlc.arg('reactions_total_count')::int, sqlc.arg('state')::text, sqlc.narg('state_reason')::text, sqlc.arg('sub_issue_total')::int, sqlc.arg('sub_issue_completed')::int, sqlc.arg('sub_issue_percent_completed')::int, sqlc.arg('issue_dependency_total_blocked_by')::int, sqlc.arg('issue_dependency_total_blocking')::int, sqlc.arg('title')::text, sqlc.narg('type_name')::text, sqlc.narg('type_description')::text, sqlc.arg('updated_at')::timestamptz, sqlc.narg('issue_created_by')::bigint, sqlc.narg('creator_name')::text, sqlc.narg('creator_type')::text,  sqlc.narg('assignee_id')::bigint, sqlc.narg('assignee_name')::text, sqlc.narg('assignee_type')::text
) ON CONFLICT (issue_id) DO NOTHING;

-- name: InsertGitHubIssue :exec
INSERT INTO github_repo_issues(id, github_repo_id, github_user_id, hook_id, event_action, assignee_id, assignee_name, assignee_type, sender_id, sender_name, sender_type)
VALUES(
    sqlc.arg('id')::uuid, sqlc.arg('github_repo_id')::bigint, sqlc.arg('github_user_id')::bigint, sqlc.arg('hook_id')::bigint, sqlc.arg('event_action')::text, sqlc.narg('assignee_id')::bigint, sqlc.narg('assignee_name')::text, sqlc.narg('assignee_type')::text, sqlc.arg('sender_id')::bigint, sqlc.arg('sender_name')::text, sqlc.arg('sender_type')::text
);

-- name: IssueInfoExistsInsideTheGitHub :one
SELECT EXISTS(
    SELECT * FROM github_repo_issues_info WHERE issue_id = $1
);

-- name: InsertGitHubIssueComment :exec
INSERT INTO github_repo_issues_comments(issue_id, action, change_from, created_at, updated_at, author_association, comment_id, commentor_id, commentor_name, commentor_type, body, positive_reactions, negative_reactions, total_reactions)
VALUES(
    sqlc.arg('issue_id')::bigint, sqlc.arg('action')::text, sqlc.narg('change_from')::text, sqlc.arg('created_at')::timestamptz, sqlc.arg('updated_at')::timestamptz, sqlc.arg('author_association')::text, sqlc.arg('comment_id')::bigint, sqlc.arg('commentor_id')::bigint, sqlc.arg('commentor_name')::text, sqlc.arg('commentor_type')::text, sqlc.arg('body')::text, sqlc.arg('positive_reactions')::int, sqlc.arg('negative_reactions')::int, sqlc.arg('total_reactions')::int
);

-- name: InsertGitHubWebhookDeliveries :execrows
INSERT INTO github_repo_webhook_deliveries(delivery_id)
VALUES(
    sqlc.arg('delivery_id')::uuid
) ON CONFLICT (delivery_id) DO NOTHING;

-- name: InsertGitHubReleases :exec
INSERT INTO github_repo_releases(github_repo_id, github_user_id, releases_id, hook_id, action, tag_name, name, body, target_commitish, draft, prerelease, published_at, sender_id, sender_name, sender_type)
VALUES(
    sqlc.arg('github_repo_id')::bigint, sqlc.arg('github_user_id')::bigint, sqlc.arg('releases_id')::bigint, sqlc.arg('hook_id')::bigint, sqlc.arg('action')::text, sqlc.arg('tag_name')::text, sqlc.arg('name')::text, sqlc.arg('body')::text, sqlc.arg('target_commitish')::text, sqlc.arg('draft')::boolean, sqlc.arg('prerelease')::boolean, sqlc.arg('published_at')::timestamptz, sqlc.arg('sender_id')::bigint, sqlc.arg('sender_name')::text, sqlc.arg('sender_type')::text
);

-- name: DeleteGitHubMock :exec
TRUNCATE TABLE github_user, github_repo, github_repo_license, github_repo_push, github_repo_push_commit, github_repo_pull_request, github_repo_pull_request_info, github_repo_issues, github_repo_issues_info, github_repo_issues_labels, github_repo_issues_field_values, github_repo_issues_multi_select_options, github_repo_issues_comments CASCADE;

-- We create an enumerate to store the roles
CREATE TYPE user_role AS ENUM ('user', 'admin', 'worker', 'owner');

-- The simple table made to store user info inside Kitanai (future server/app)
CREATE TABLE users (
    id UUID DEFAULT gen_random_uuid(),
    CONSTRAINT users_id_pkey PRIMARY KEY (id),
    name TEXT NOT NULL,
    CONSTRAINT users_name_key UNIQUE (name),
    CONSTRAINT users_name_check CHECK (length(trim(name)) BETWEEN 3 AND 25),
    display_name TEXT,
    CONSTRAINT users_display_name_check CHECK (display_name IS NULL OR length(trim(display_name)) BETWEEN 1 AND 30),
    bio TEXT,
    CONSTRAINT users_bio_check CHECK (bio IS NULL OR length(trim(bio)) <= 250),
    sex TEXT NOT NULL,
    CONSTRAINT users_sex_check CHECK (sex IN ('male', 'female', 'prefer_not_to_specify')),
    location TEXT,
    -- We place it forced to have this format: 'Country Code (Alpha-2), City'
    -- The regex checks structural formatting (case-sensitive by default with ~):
        -- First part  -> Matches exactly 2 uppercase letters [A-Z]{2}, followed by a literal comma and a single space (, ).
        -- Second part -> Matches the city/region name using 1 or more alphabetic characters (from more languages), spaces, dots, or hyphens [[:alpha:]\s\.-]+, from the start (^) to the end ($) of the string.
    CONSTRAINT users_location_check CHECK (location IS NULL OR (length(trim(location)) BETWEEN 5 AND 62 AND location ~ '^[A-Z]{2},\s[[:alpha:]\s\.-]+$')),
    birth_date DATE NOT NULL,
    email TEXT NOT NULL,
    CONSTRAINT users_email_key UNIQUE (email),
    -- The regex is case-insensitive (~*):
        -- First part  -> Matches 1 to 64 alphanumeric characters, dots, hyphens, or plus signs [a-z0-9._+-]{1,64}, followed by an '@' symbol.
        -- Second part -> Matches one or more domain segments (+). Each segment has 1 to 63 alphanumeric characters or hyphens [a-z0-9-]{1,63}, followed by a literal dot (\.).
        -- Third part  -> Matches a 2 to 18 character alphanumeric top-level domain [a-z0-9]{2,18} at the end of the string ($).
    CONSTRAINT users_email_check CHECK (length(trim(email)) BETWEEN 6 AND 254 AND email ~* '^[a-z0-9._+-]{1,64}@([a-z0-9-]{1,63}\.)+([a-z0-9]{2,18})$'),
    password_hash TEXT NOT NULL,
    CONSTRAINT users_password_hash_check CHECK (length(password_hash) >= 60),
    role user_role NOT NULL DEFAULT 'user',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- We create a table for pending_deletions - made by workers
CREATE TABLE pending_deletions (
    user_id UUID NOT NULL,
    CONSTRAINT pending_deletions_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    requested_by UUID NOT NULL,
    CONSTRAINT pending_deletions_requested_by_fkey FOREIGN KEY (requested_by) REFERENCES users(id),
    clarification TEXT,
    CONSTRAINT pending_deletions_clarification_check CHECK (length(trim(clarification)) <= 1024),
    requested_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    scheduled_at TIMESTAMPTZ NOT NULL,
    is_processed BOOLEAN NOT NULL DEFAULT FALSE
);

-- We create this table just to hold the info about the repo's license, nothing else.
CREATE TABLE github_repo_license(
    license_key TEXT NOT NULL,
    CONSTRAINT github_license_license_key_pkey PRIMARY KEY (license_key),
    name TEXT NOT NULL,
    spdx_id TEXT NOT NULL,
    url TEXT,
    node_id TEXT NOT NULL
);

-- A simple table containing the GitHub information about the user
CREATE TABLE github_user(
    user_id UUID NOT NULL,
    CONSTRAINT github_user_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    github_id BIGINT NOT NULL,
    CONSTRAINT github_user_github_id_pkey PRIMARY KEY (github_id),
    name TEXT NOT NULL,
    CONSTRAINT github_user_github_name_key UNIQUE (name),
    avatar_url TEXT NOT NULL,
    html_url TEXT NOT NULL,
    company TEXT,
    email TEXT,
    hireable BOOLEAN,
    bio TEXT,
    followers INT DEFAULT 0,
    total_public_repos SMALLINT NOT NULL,
    total_private_repos SMALLINT NOT NULL,
    total_repos SMALLINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

-- A simple table to hold the repositories of the user
CREATE TABLE github_repo(
    github_id BIGINT NOT NULL,
    CONSTRAINT github_repo_github_user_id_fkey FOREIGN KEY (github_id) REFERENCES github_user(github_id),
    repo_id BIGINT NOT NULL,
    CONSTRAINT github_repo_repo_id_pkey PRIMARY KEY (repo_id),
    name TEXT NOT NULL,
    html_url TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    pushed_at TIMESTAMPTZ NOT NULL,
    stars INT DEFAULT 0,
    watches INT DEFAULT 0,
    forks_count INT DEFAULT 0,
    main_language TEXT,
    license_key TEXT,
    CONSTRAINT github_repo_license_key_fkey FOREIGN KEY (license_key) REFERENCES github_repo_license(license_key) ON DELETE SET NULL,
    topic TEXT[] NOT NULL DEFAULT '{}',
    archived BOOLEAN NOT NULL,
    visibility TEXT NOT NULL
);

CREATE TABLE github_repo_webhook_deliveries(
    delivery_id UUID NOT NULL,
    CONSTRAINT github_repo_webhook_deliveries_delivery_id_pkey PRIMARY KEY (delivery_id)
);

CREATE TABLE github_repo_push(
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    CONSTRAINT github_repo_push_id PRIMARY KEY (id),
    github_repo_id BIGINT NOT NULL,
    CONSTRAINT github_repo_push_github_repo_id_fkey FOREIGN KEY (github_repo_id) REFERENCES github_repo(repo_id) ON DELETE CASCADE,
    github_user_id BIGINT NOT NULL,
    CONSTRAINT github_repo_push_github_user_id_fkey FOREIGN KEY (github_user_id) REFERENCES github_user(github_id) ON DELETE CASCADE,
    hook_id BIGINT NOT NULL,
    full_name TEXT NOT NULL,
    ref TEXT NOT NULL,
    before_sha TEXT,
    after_sha TEXT,
    head_commit_id TEXT,
    compare TEXT NOT NULL,
    forced boolean NOT NULL DEFAULT false,
    created boolean NOT NULL DEFAULT false,
    deleted boolean NOT NULL DEFAULT false,
    pushed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE github_repo_push_commit(
    push_id UUID NOT NULL,
    CONSTRAINT github_repo_push_commit_push_id_fkey FOREIGN KEY (push_id) REFERENCES github_repo_push(id) ON DELETE CASCADE,
    commit_sha TEXT NOT NULL,
    CONSTRAINT github_repo_push_commit_commit_sha_pkey PRIMARY KEY (commit_sha),
    message TEXT NOT NULL,
    added TEXT[] DEFAULT '{}',
    removed TEXT[] DEFAULT '{}',
    modified TEXT[] DEFAULT '{}',
    url TEXT NOT NULL,
    committed_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE github_repo_pull_request_info(
    id BIGINT NOT NULL,
    CONSTRAINT github_repo_pull_request_info_id_pkey PRIMARY KEY (id),
    number INT NOT NULL,
    title TEXT NOT NULL,
    state TEXT NOT NULL,
    is_draft BOOLEAN NOT NULL,
    is_merged BOOLEAN,
    author_id BIGINT,
    author_name TEXT,
    author_type TEXT
);

CREATE TABLE github_repo_pull_request(
    github_repo_id BIGINT NOT NULL,
    CONSTRAINT github_repo_pull_request_github_repo_id_fkey FOREIGN KEY (github_repo_id) REFERENCES github_repo(repo_id),
    github_user_id BIGINT NOT NULL,
    CONSTRAINT github_repo_pull_request_github_user_id_fkey FOREIGN KEY (github_user_id) REFERENCES github_user(github_id),
    hook_id BIGINT NOT NULL,
    event_action TEXT NOT NULL,
    full_name TEXT NOT NULL,
    sender_id BIGINT NOT NULL,
    sender_name TEXT NOT NULL,
    sender_type TEXT NOT NULL,
    pull_request_id BIGINT NOT NULL,
    CONSTRAINT github_repo_pull_request_pull_request_id_fkey FOREIGN KEY (pull_request_id) REFERENCES github_repo_pull_request_info(id),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    closed_at TIMESTAMPTZ,
    merged_at TIMESTAMPTZ,
    additions INT NOT NULL,
    deletions INT NOT NULL,
    changed_files INT NOT NULL,
    commits_count INT NOT NULL,
    comments_count INT NOT NULL,
    review_comments_count INT NOT NULL,
    head_branch TEXT NOT NULL,
    head_sha TEXT NOT NULL,
    base_branch TEXT NOT NULL,
    merge_commit_sha TEXT,
    assignee_ids BIGINT[] DEFAULT '{}',
    requested_reviewer_ids BIGINT[] DEFAULT '{}',
    requested_team_ids BIGINT[] DEFAULT '{}',
    labels TEXT[] DEFAULT '{}',
    milestone_id BIGINT
);

CREATE TABLE github_repo_issues(
    id UUID NOT NULL,
    CONSTRAINT github_repo_issues_id_pkey PRIMARY KEY (id),
    github_repo_id BIGINT NOT NULL,
    CONSTRAINT github_repo_issues_github_repo_id_fkey FOREIGN KEY (github_repo_id) REFERENCES github_repo(repo_id),
    github_user_id BIGINT NOT NULL,
    CONSTRAINT github_repo_issues_github_user_id_fkey FOREIGN KEY (github_user_id) REFERENCES github_user(github_id),
    hook_id BIGINT NOT NULL,
    event_action TEXT NOT NULL,
    assignee_id BIGINT,
    assignee_name TEXT,
    assignee_type TEXT,
    sender_id BIGINT NOT NULL,
    sender_name TEXT NOT NULL,
    sender_type TEXT NOT NULL
);

CREATE TABLE github_repo_issues_info(
    related_id UUID NOT NULL,
    CONSTRAINT github_repo_issues_info_related_id_fkey FOREIGN KEY (related_id) REFERENCES github_repo_issues(id) ON DELETE CASCADE,
    CONSTRAINT github_repo_issues_info_related_id_key UNIQUE (related_id),
    issue_id BIGINT NOT NULL,
    CONSTRAINT github_repo_issues_info_pkey PRIMARY KEY (issue_id),
    author_association TEXT NOT NULL,
    body TEXT,
    comments INT NOT NULL,
    draft BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ,
    locked BOOLEAN NOT NULL,
    milestone_description TEXT,
    milestone_due_on TEXT,
    milestone_state TEXT,
    milestone_title TEXT,
    number INT NOT NULL,
    positive_reactions INT NOT NULL,
    negative_reactions INT NOT NULL,
    reactions_total_count INT NOT NULL,
    state TEXT NOT NULL,
    state_reason TEXT,
    sub_issue_total INT NOT NULL,
    sub_issue_completed INT NOT NULL,
    sub_issue_percent_completed INT NOT NULL,
    issue_dependency_total_blocked_by INT NOT NULL,
    issue_dependency_total_blocking INT NOT NULL,
    title TEXT NOT NULL,
    type_name TEXT,
    type_description TEXT,
    updated_at TIMESTAMPTZ NOT NULL,
    issue_created_by BIGINT,
    creator_name TEXT,
    creator_type TEXT,
    assignee_id BIGINT,
    assignee_name TEXT,
    assignee_type TEXT
);

CREATE TABLE github_repo_issues_labels(
    issue_id BIGINT NOT NULL,
    CONSTRAINT github_repo_issues_label_issue_id_fkey FOREIGN KEY (issue_id) REFERENCES github_repo_issues_info(issue_id) ON DELETE CASCADE,
    label_id BIGINT NOT NULL,
    CONSTRAINT github_repo_issues_labels_label_issue_key UNIQUE (issue_id, label_id),
    name TEXT NOT NULL,
    description TEXT
);

CREATE TABLE github_repo_issues_field_values(
    id UUID NOT NULL,
    CONSTRAINT github_repo_issues_field_values_id_pkey PRIMARY KEY (id),
    issue_id BIGINT NOT NULL,
    CONSTRAINT github_repo_issues_field_values_issue_id_fkey FOREIGN KEY (issue_id) REFERENCES github_repo_issues_info(issue_id) ON DELETE CASCADE,
    issue_field_name TEXT NOT NULL,
    data_type TEXT NOT NULL,
    value JSONB,
    single_select_option_id BIGINT,
    single_select_option_name TEXT
);

CREATE TABLE github_repo_issues_multi_select_options(
    field_value_id UUID NOT NULL,
    CONSTRAINT github_repo_issues_multi_select_options_field_value_id_fkey FOREIGN KEY (field_value_id) REFERENCES github_repo_issues_field_values(id) ON DELETE CASCADE,
    id BIGINT NOT NULL,
    CONSTRAINT mgithub_repo_issues_multi_select_options_id_pkey PRIMARY KEY (id),
    name TEXT NOT NULL
);

CREATE TABLE github_repo_issues_comments(
    issue_id BIGINT NOT NULL,
    CONSTRAINT github_repo_issues_comments_issue_id_fkey FOREIGN KEY (issue_id) REFERENCES github_repo_issues_info(issue_id) ON DELETE CASCADE,
    action TEXT NOT NULL,
    change_from TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    author_association TEXT NOT NULL,
    comment_id BIGINT NOT NULL,
    commentor_id BIGINT NOT NULL,
    commentor_name TEXT NOT NULL,
    commentor_type TEXT NOT NULL,
    body TEXT NOT NULL,
    positive_reactions INT NOT NULL,
    negative_reactions INT NOT NULL,
    total_reactions INT NOT NULL
);

CREATE TABLE github_repo_releases(
    github_repo_id BIGINT NOT NULL,
    CONSTRAINT github_repo_releases_github_repo_id_fkey FOREIGN KEY (github_repo_id) REFERENCES github_repo(repo_id) ON DELETE CASCADE,
    github_user_id BIGINT NOT NULL,
    CONSTRAINT github_repo_releases_github_user_id_fkey FOREIGN KEY (github_user_id) REFERENCES github_user(github_id) ON DELETE CASCADE,
    releases_id BIGINT NOT NULL,
    CONSTRAINT github_repo_releases_releases_id_pkey PRIMARY KEY (releases_id),
    hook_id BIGINT NOT NULL,
    action TEXT NOT NULL,
    tag_name TEXT NOT NULL,
    name TEXT NOT NULL,
    body TEXT NOT NULL,
    target_commitish TEXT NOT NULL,
    draft BOOLEAN NOT NULL,
    prerelease BOOLEAN NOT NULL,
    published_at TIMESTAMPTZ,
    sender_id BIGINT NOT NULL,
    sender_name TEXT NOT NULL,
    sender_type TEXT NOT NULL
);

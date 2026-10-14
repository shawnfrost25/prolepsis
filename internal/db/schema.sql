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
     ---------- IDs ----------
     spdx_id TEXT,
     node_id TEXT NOT NULL,

     ---------- GENERAL INFO TEXT ----------
     license_key TEXT NOT NULL,
     CONSTRAINT github_license_license_key_pkey PRIMARY KEY (license_key),
     name TEXT NOT NULL,
     body TEXT NOT NULL,

     ---------- GENERAL INFO NUM ----------

     ---------- GENERAL INFO BOOL ----------

     ---------- URLs ----------
     html_url TEXT

     ---------- GENERAL INFO TIME ----------
);

-- A simple table containing the GitHub information about the user
CREATE TABLE github_user(
    ---------- IDs ----------
    user_id UUID NOT NULL,
    CONSTRAINT github_user_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    github_id BIGINT NOT NULL,
    CONSTRAINT github_user_github_id_pkey PRIMARY KEY (github_id),

    ---------- GENERAL INFO TEXT ----------
    login TEXT NOT NULL,
    CONSTRAINT github_user_github_name_key UNIQUE (login),
    company TEXT,
    email TEXT,
    bio TEXT,
    status_message TEXT,

    ---------- GENERAL INFO NUM ----------
    followers INT NOT NULL DEFAULT 0,
    total_repos SMALLINT NOT NULL,

    ---------- GENERAL INFO BOOL ----------
    hireable BOOLEAN NOT NULL,

    ---------- URLs ----------
    avatar_url TEXT NOT NULL,
    html_url TEXT NOT NULL,

    ---------- GENERAL INFO TIME ----------
    created_at TIMESTAMPTZ NOT NULL
);

-- A simple table to hold the repositories of the user
CREATE TABLE github_repo(
    ---------- IDs ----------
    github_id BIGINT NOT NULL,
    CONSTRAINT github_repo_github_id_fkey FOREIGN KEY (github_id) REFERENCES github_user(github_id),
    repo_id BIGINT NOT NULL,
    CONSTRAINT github_repo_repo_id_pkey PRIMARY KEY (repo_id),
    hook_id BIGINT,
    CONSTRAINT github_repo_hook_id_key UNIQUE (hook_id),

    ---------- GENERAL INFO TEXT ----------
    name TEXT NOT NULL,
    description TEXT,
    readme TEXT,
    main_language TEXT,
    secondary_languages TEXT[],
    license_key TEXT,
    CONSTRAINT github_repo_license_key_fkey FOREIGN KEY (license_key) REFERENCES github_repo_license(license_key) ON DELETE SET NULL,
    topic TEXT[] DEFAULT '{}',
    visibility TEXT NOT NULL,

    ---------- GENERAL INFO NUM ----------
    stars INT NOT NULL DEFAULT 0,
    watches INT NOT NULL DEFAULT 0,
    forks_count INT NOT NULL DEFAULT 0,
    total_releases INT NOT NULL,
    
    ---------- GENERAL INFO BOOL ----------
    archived BOOLEAN NOT NULL,
    
    ---------- URLs ----------
    html_url TEXT NOT NULL,

    ---------- GENERAL INFO TIME ----------
    created_at TIMESTAMPTZ NOT NULL,
    pushed_at TIMESTAMPTZ
);

CREATE TABLE github_repo_webhook_deliveries(
    delivery_id UUID NOT NULL,
    CONSTRAINT github_repo_webhook_deliveries_delivery_id_pkey PRIMARY KEY (delivery_id)
);

CREATE TABLE pull_requests(
    ---------- IDs ----------
    id BIGINT NOT NULL,
    CONSTRAINT pull_requests_id_pkey PRIMARY KEY (id),
    github_repo_id BIGINT NOT NULL,
    CONSTRAINT pull_requests_github_repo_id_fkey FOREIGN KEY (github_repo_id) REFERENCES github_repo(repo_id),
    github_user_id BIGINT NOT NULL,
    CONSTRAINT pull_requests_github_user_id_fkey FOREIGN KEY (github_user_id) REFERENCES github_user(github_id),

    ---------- GENERAL INFO TEXT ----------
    action TEXT,
    author_login TEXT,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    state TEXT NOT NULL,
    labels TEXT[],

   ---------- GENERAL INFO NUM ----------
   additions INT NOT NULL,
   changed_files INT NOT NULL,
   deletions INT NOT NULL,
   number INT NOT NULL,
   participants_number INT NOT NULL,
   positive_reactions INT NOT NULL,
   negative_reactions INT NOT NULL,
   total_reactions INT NOT NULL,

   ---------- GENERAL INFO BOOL ----------
   is_closed BOOLEAN NOT NULL,
   is_draft BOOLEAN NOT NULL,
   is_locked BOOLEAN NOT NULL,
   is_merged BOOLEAN NOT NULL,

   ---------- URLs ----------
   html_url TEXT NOT NULL,

   ---------- GENERAL INFO TIME ----------
   created_at TIMESTAMPTZ NOT NULL,
   updated_at TIMESTAMPTZ NOT NULL,
   closed_at TIMESTAMPTZ,
   merged_at TIMESTAMPTZ
);

CREATE TABLE pull_request_comments(
    ---------- IDs ----------
    id BIGINT NOT NULL,
    CONSTRAINT pull_request_comments_id_pkey PRIMARY KEY (id),
    pr_id BIGINT NOT NULL,
    CONSTRAINT pull_request_comments_pr_id_fkey FOREIGN KEY (pr_id) REFERENCES pull_requests(id) ON DELETE CASCADE,

    ---------- GENERAL INFO TEXT ----------
    action TEXT,
    author_login TEXT,
    body TEXT NOT NULL,
    minimized_reason TEXT,

    ---------- GENERAL INFO NUM ----------

    ---------- GENERAL INFO BOOL ----------
    is_minimized BOOLEAN NOT NULL,

    ---------- URLs ----------
    html_url TEXT NOT NULL,

    ---------- GENERAL INFO TIME ----------
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    last_edited_at TIMESTAMPTZ
);

CREATE TABLE issue_requests(
    ---------- IDs ----------
    id BIGINT NOT NULL,
    CONSTRAINT issue_requests_id_pkey PRIMARY KEY (id),
    github_repo_id BIGINT NOT NULL,
    CONSTRAINT issue_requests_github_repo_id_fkey FOREIGN KEY (github_repo_id) REFERENCES github_repo(repo_id),
    github_user_id BIGINT NOT NULL,
    CONSTRAINT issue_requests_github_user_id_fkey FOREIGN KEY (github_user_id) REFERENCES github_user(github_id),
    
    ---------- GENERAL INFO TEXT ----------
    action TEXT,
    author_login TEXT,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    state TEXT NOT NULL,
    state_reason TEXT,
    issue_type TEXT,
    labels TEXT[],

    ---------- GENERAL INFO NUM ----------
    number INT NOT NULL,
    participants_number INT NOT NULL,
    positive_reactions INT NOT NULL,
    negative_reactions INT NOT NULL,
    total_reactions INT NOT NULL,

    ---------- GENERAL INFO BOOL ----------
    is_closed BOOLEAN,
    is_locked BOOLEAN,

    ---------- URLs ----------
    html_url TEXT NOT NULL,

    ---------- GENERAL INFO TIME ----------
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    closed_at TIMESTAMPTZ,
    last_edited_at TIMESTAMPTZ
);

CREATE TABLE issue_comments(
    ---------- IDs ----------
    id BIGINT NOT NULL,
    CONSTRAINT issue_comments_id_pkey PRIMARY KEY (id),
    issue_id BIGINT NOT NULL,
    CONSTRAINT issue_comments_issue_id_fkey FOREIGN KEY (issue_id) REFERENCES issue_requests(id) ON DELETE CASCADE,

    ---------- GENERAL INFO TEXT ----------
    action TEXT,
    author_login TEXT,
    body TEXT NOT NULL,
    minimized_reason TEXT,

    ---------- GENERAL INFO NUM ----------

    ---------- GENERAL INFO BOOL ----------
    is_minimized BOOLEAN NOT NULL,

    ---------- URLs ----------
    html_url TEXT NOT NULL,

    ---------- GENERAL INFO TIME ----------
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    last_edited_at TIMESTAMPTZ
);

CREATE TABLE release_requests(
    ---------- IDs ----------
    id BIGINT NOT NULL,
    CONSTRAINT release_requests_id_pkey PRIMARY KEY (id),
    github_repo_id BIGINT NOT NULL,
    CONSTRAINT release_requests_github_repo_id_fkey FOREIGN KEY (github_repo_id) REFERENCES github_repo(repo_id) ON DELETE CASCADE,
    github_user_id BIGINT NOT NULL,
    CONSTRAINT release_requests_github_user_id_fkey FOREIGN KEY (github_user_id) REFERENCES github_user(github_id) ON DELETE CASCADE,

    ---------- GENERAL INFO TEXT ---------
    action TEXT,
    author_login TEXT,
    name TEXT,
    tag_name TEXT,
    description TEXT,

    ---------- GENERAL INFO NUM ---------

    ---------- GENERAL INFO BOOL ---------
    is_draft BOOLEAN NOT NULL,
    is_prereleased BOOLEAN NOT NULL,
    is_immutable BOOLEAN NOT NULL,

    ---------- URLs ---------
    html_url TEXT NOT NULL,

    ---------- GENERAL INFO TIME ---------
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    published_at TIMESTAMPTZ
);

CREATE TABLE push_webhook(
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

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
    clarification TEXT NOT NULL,
    CONSTRAINT pending_deletions_clarification_check CHECK (length(trim(clarification)) <= 1024),
    requested_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    scheduled_at TIMESTAMPTZ NOT NULL,
    is_processed BOOLEAN NOT NULL DEFAULT FALSE
);

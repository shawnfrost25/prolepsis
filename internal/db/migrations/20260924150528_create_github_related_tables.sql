-- Create "github_user" table
CREATE TABLE "public"."github_user" (
  "user_id" uuid NOT NULL,
  "github_id" bigint NOT NULL,
  "name" text NOT NULL,
  "avatar_url" text NOT NULL,
  "html_url" text NOT NULL,
  "company" text NULL,
  "email" text NULL,
  "hireable" boolean NULL,
  "bio" text NULL,
  "followers" integer NULL DEFAULT 0,
  "total_public_repos" smallint NOT NULL,
  "total_private_repos" smallint NOT NULL,
  "total_repos" smallint NOT NULL,
  "created_at" timestamptz NOT NULL,
  CONSTRAINT "github_user_github_id_pkey" PRIMARY KEY ("github_id"),
  CONSTRAINT "github_user_github_name_key" UNIQUE ("name"),
  CONSTRAINT "github_user_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create "github_repo_license" table
CREATE TABLE "public"."github_repo_license" (
  "license_key" text NOT NULL,
  "name" text NOT NULL,
  "spdx_id" text NOT NULL,
  "url" text NULL,
  "node_id" text NOT NULL,
  CONSTRAINT "github_license_license_key_pkey" PRIMARY KEY ("license_key")
);
-- Create "github_repo" table
CREATE TABLE "public"."github_repo" (
  "github_id" bigint NOT NULL,
  "repo_id" bigint NOT NULL,
  "name" text NOT NULL,
  "html_url" text NOT NULL,
  "description" text NULL,
  "created_at" timestamptz NOT NULL,
  "pushed_at" timestamptz NOT NULL,
  "stars" integer NULL DEFAULT 0,
  "watches" integer NULL DEFAULT 0,
  "forks_count" integer NULL DEFAULT 0,
  "main_language" text NULL,
  "license_key" text NULL,
  "topic" text[] NOT NULL DEFAULT '{}',
  "archived" boolean NOT NULL,
  "visibility" text NOT NULL,
  CONSTRAINT "github_repo_repo_id_pkey" PRIMARY KEY ("repo_id"),
  CONSTRAINT "github_repo_github_user_id_fkey" FOREIGN KEY ("github_id") REFERENCES "public"."github_user" ("github_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "github_repo_license_key_fkey" FOREIGN KEY ("license_key") REFERENCES "public"."github_repo_license" ("license_key") ON UPDATE NO ACTION ON DELETE SET NULL
);

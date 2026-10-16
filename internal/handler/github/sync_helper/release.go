package synchelper

import (
	"context"
	"net/http"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/handler/github/webhook"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
)

func ReleaseCallback(logger zerolog.Logger, release webhook.ReleaseNode, ctx context.Context, githubRepoId int64, githubUserId int64, queries *db.Queries) bool {
	logger = logger.With().Str("process", "releases").Logger()
	if release.DatabaseId == nil {
		logger = logger.With().Str("release_name", release.TagName).Int("release_id", *release.DatabaseId).Logger()
		logger.Debug().
			Int("status", http.StatusBadRequest).
			Str("code", "release_id_missing").
			Msg("skip release")
		return true
	}

	var authorLogin *string
	if release.Author != nil {
		authorLogin = &release.Author.Login
	}

	var publishedAt pgtype.Timestamptz
	if release.PublishedAt != nil {
		publishedAt = pgtype.Timestamptz{Time: *release.PublishedAt, Valid: true}
	}

	err := queries.InsertRelease(ctx, db.InsertReleaseParams{
		ID:            int64(*release.DatabaseId),
		GithubRepoID:  githubRepoId,
		GithubUserID:  githubUserId,
		AuthorLogin:   authorLogin,
		Name:          &release.TagName,
		TagName:       release.TagName,
		Description:   release.Description,
		IsDraft:       release.IsDraft,
		IsPrereleased: release.IsPrereleased,
		IsImmutable:   release.IsImmutable,
		HtmlUrl:       release.HtmlUrl,
		CreatedAt:     pgtype.Timestamptz{Time: release.CreatedAt},
		UpdatedAt:     pgtype.Timestamptz{Time: release.UpdatedAt},
		PublishedAt:   publishedAt,
	})
	if err != nil {
		logger = logger.With().Str("release_name", release.TagName).Int("release_id", *release.DatabaseId).Logger()
		logger.Debug().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("code", "release_insert_failed").
			Msg("skip release")
		return true
	}
	return false
}

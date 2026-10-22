package synchelper

import (
	"context"
	"net/http"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/failure"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
)

func PullRequestCallback(logger zerolog.Logger, pullRequest PullRequestNode, ctx context.Context, githubRepoId int64, githubUserId int64, queries *db.Queries) bool {
	logger = logger.With().Str("process", "pull_request").Logger()
	pullRequestId, err, code := ParseId(pullRequest.FullDatabaseId)
	if err != nil {
		err = failure.Wrap("github.http.pull_request.id.validate", err)
		logger = logger.With().Str("pull_request_name", pullRequest.Title).Interface("pull_request_id", pullRequest.FullDatabaseId).Logger()
		logger.Debug().
			Err(err).
			Int("status", http.StatusBadRequest).
			Str("code", "pull_request"+code).
			Msg("skip pull request")
		return true
	}

	switch pullRequest.State {
	case "OPEN", "MERGED", "CLOSED":

		var authorLogin *string
		if pullRequest.Author != nil {
			authorLogin = &pullRequest.Author.Login
		}

		var labels []string
		if len(pullRequest.Labels.Nodes) != 0 {
			for _, label := range pullRequest.Labels.Nodes {
				if label.Name == "" {
					continue
				}
				labels = append(labels, label.Name)
			}
		}

		positiveReactions := 0
		negativeReactions := 0

		for _, reaction := range pullRequest.ReactionGroups {
			switch reaction.Content {
			case "THUMBS_UP", "HEART", "HOORAY", "LAUGH", "ROCKET":
				positiveReactions += reaction.Reactors.TotalCount
			case "THUMBS_DOWN", "CONFUSED":
				negativeReactions += reaction.Reactors.TotalCount
			}
		}
		totalCount := positiveReactions + negativeReactions

		var closedAt pgtype.Timestamptz
		if pullRequest.ClosedAt != nil {
			closedAt = pgtype.Timestamptz{Time: *pullRequest.ClosedAt, Valid: true}
		}
		var mergedAt pgtype.Timestamptz
		if pullRequest.MergedAt != nil {
			mergedAt = pgtype.Timestamptz{Time: *pullRequest.MergedAt, Valid: true}
		}
		var body *string
		if pullRequest.Body != "" {
			body = &pullRequest.Body
		}
		parseInt := func(number int32) *int32 {
			return &number
		}
		err := queries.InsertPullRequest(ctx, db.InsertPullRequestParams{
			ID:                 pullRequestId,
			GithubRepoID:       githubRepoId,
			GithubUserID:       githubUserId,
			AuthorLogin:        authorLogin,
			Title:              pullRequest.Title,
			Body:               body,
			State:              pullRequest.State,
			Labels:             labels,
			Additions:          int32(pullRequest.Additions),
			ChangedFiles:       int32(pullRequest.ChangedFiles),
			Deletions:          int32(pullRequest.Deletions),
			Number:             int32(pullRequest.Number),
			ParticipantsNumber: parseInt(int32(pullRequest.Participants.TotalCount)),
			PositiveReactions:  parseInt(int32(positiveReactions)),
			NegativeReactions:  parseInt(int32(negativeReactions)),
			TotalReactions:     parseInt(int32(totalCount)),
			IsClosed:           pullRequest.IsClosed,
			IsDraft:            pullRequest.IsDraft,
			IsLocked:           pullRequest.IsLocked,
			IsMerged:           pullRequest.IsMerged,
			HtmlUrl:            pullRequest.HtmlUrl,
			CreatedAt:          pgtype.Timestamptz{Time: pullRequest.CreatedAt, Valid: true},
			UpdatedAt:          pgtype.Timestamptz{Time: pullRequest.UpdatedAt, Valid: true},
			ClosedAt:           closedAt,
			MergedAt:           mergedAt,
		})
		if err != nil {
			err = failure.Wrap("github.database.pull_request.insert", err)
			logger = logger.With().Str("pull_request_name", pullRequest.Title).Str("pull_request_id", *pullRequest.FullDatabaseId).Logger()
			logger.Debug().
				Err(err).
				Int("status", http.StatusInternalServerError).
				Str("code", "pull_request_insert_failed").
				Msg("skip pull request")
			return true
		}

		if len(pullRequest.Comments.Nodes) != 0 {
			for _, comment := range pullRequest.Comments.Nodes {
				var authorLogin *string
				if comment.Author != nil {
					authorLogin = &comment.Author.Login
				}

				pullRequestCommentId, err, code := ParseId(comment.FullDatabaseId)
				if err != nil {
					err = failure.Wrap("github.http.comment.id.validate", err)
					logger = logger.With().Interface("pull_request_comment_id", comment.FullDatabaseId).Logger()
					logger.Debug().
						Err(err).
						Int("status", http.StatusBadRequest).
						Str("code", "pull_request_comment"+code).
						Msg("skip pull request comment")
					return true
				}

				err = queries.InsertPullRequestComment(ctx, db.InsertPullRequestCommentParams{
					ID:              pullRequestCommentId,
					PrID:            pullRequestId,
					AuthorLogin:     authorLogin,
					Body:            comment.Body,
					MinimizedReason: comment.MinimizedReason,
					HtmlUrl:         comment.HtmlUrl,
					CreatedAt:       pgtype.Timestamptz{Time: comment.CreatedAt, Valid: true},
					UpdatedAt:       pgtype.Timestamptz{Time: comment.UpdatedAt, Valid: true},
				})
				if err != nil {
					err = failure.Wrap("github.database.pull_request.comment.insert", err)
					logger.Debug().
						Err(err).
						Int("status", http.StatusInternalServerError).
						Str("code", "pull_request_comment_insert_failed").
						Msg("skip pull request comment")
					return true
				}
			}
		}
	default:
		err = failure.Invalid("pull_request_state", "unsupported state "+pullRequest.State)
		err = failure.Wrap("github.http.pull_request.state.validate", err)
		logger = logger.With().Str("pull_request_name", pullRequest.Title).Str("pull_request_id", *pullRequest.FullDatabaseId).Logger()
		logger.Debug().
			Err(err).
			Int("status", http.StatusBadRequest).
			Str("code", "pull_request_state_invalid").
			Str("pull_request_state", pullRequest.State).
			Msg("skip pull request")
		return true
	}
	return false
}

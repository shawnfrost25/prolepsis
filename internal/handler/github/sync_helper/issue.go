package synchelper

import (
	"context"
	"net/http"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/failure"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
)

func IssueCallback(logger zerolog.Logger, issue IssueNode, ctx context.Context, githubRepoId int64, githubUserId int64, queries *db.Queries) bool {
	logger = logger.With().Str("process", "issues").Logger()
	issueId, err, code := ParseId(issue.FullDatabaseId)
	if err != nil {
		err = failure.Wrap("github.http.issue.id.validate", err)
		logger = logger.With().Str("issue_name", issue.Title).Interface("issue_id", issue.FullDatabaseId).Logger()
		logger.Debug().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("code", "issue"+code).
			Msg("skip issue")
		return true
	}

	switch issue.State {
	case "CLOSED", "OPEN":

		var authorLogin *string
		if issue.Author != nil {
			authorLogin = &issue.Author.Login
		}

		var issueType *string
		if issue.IssueType != nil {
			issueType = &issue.IssueType.Name
		}

		var labels []string
		if len(issue.Labels.Nodes) != 0 {
			for _, label := range issue.Labels.Nodes {
				if label.Name == "" {
					continue
				}
				labels = append(labels, label.Name)
			}
		}

		positiveReactions := 0
		negativeReactions := 0

		for _, reaction := range issue.ReactionGroups {
			switch reaction.Content {
			case "THUMBS_UP", "HEART", "HOORAY", "LAUGH", "ROCKET":
				positiveReactions += reaction.Reactors.TotalCount
			case "THUMBS_DOWN", "CONFUSED":
				negativeReactions += reaction.Reactors.TotalCount
			}
		}
		totalCount := positiveReactions + negativeReactions

		var closedAt pgtype.Timestamptz
		if issue.ClosedAt != nil {
			closedAt = pgtype.Timestamptz{Time: *issue.ClosedAt, Valid: true}
		}

		err := queries.InsertIssue(ctx, db.InsertIssueParams{
			ID:                 issueId,
			GithubRepoID:       githubRepoId,
			GithubUserID:       githubUserId,
			AuthorLogin:        authorLogin,
			Title:              issue.Title,
			Body:               issue.Body,
			State:              issue.State,
			StateReason:        issue.StateReason,
			IssueType:          issueType,
			Labels:             labels,
			Number:             int32(issue.Number),
			ParticipantsNumber: int32(issue.Participants.TotalCount),
			PositiveReactions:  int32(positiveReactions),
			NegativeReactions:  int32(negativeReactions),
			TotalReactions:     int32(totalCount),
			IsClosed:           issue.IsClosed,
			IsLocked:           issue.IsLocked,
			HtmlUrl:            issue.HtmlUrl,
			CreatedAt:          pgtype.Timestamptz{Time: issue.CreatedAt, Valid: true},
			UpdatedAt:          pgtype.Timestamptz{Time: issue.UpdatedAt, Valid: true},
			ClosedAt:           closedAt,
		})
		if err != nil {
			err = failure.Wrap("github.database.issue.insert", err)
			logger = logger.With().Str("issue_name", issue.Title).Str("issue_id", *issue.FullDatabaseId).Logger()
			logger.Debug().
				Err(err).
				Int("status", http.StatusInternalServerError).
				Str("code", "issue_insert_failed").
				Msg("skip issue")
			return true
		}

		if len(issue.Comments.Nodes) != 0 {
			for _, comment := range issue.Comments.Nodes {
				var authorLogin *string
				if comment.Author != nil {
					authorLogin = &comment.Author.Login
				}

				issueCommentId, err, code := ParseId(comment.FullDatabaseId)
				if err != nil {
					err = failure.Wrap("github.http.comment.id.validate", err)
					logger = logger.With().Interface("issue_comment_id", comment.FullDatabaseId).Logger()
					logger.Debug().
						Err(err).
						Int("status", http.StatusBadRequest).
						Str("code", "issue_comment"+code).
						Msg("skip issue comment")
					continue
				}

				err = queries.InsertIssueComment(ctx, db.InsertIssueCommentParams{
					ID:              issueCommentId,
					IssueID:         issueId,
					AuthorLogin:     authorLogin,
					Body:            comment.Body,
					MinimizedReason: comment.MinimizedReason,
					HtmlUrl:         comment.HtmlUrl,
					CreatedAt:       pgtype.Timestamptz{Time: comment.CreatedAt, Valid: true},
					UpdatedAt:       pgtype.Timestamptz{Time: comment.UpdatedAt, Valid: true},
				})
				if err != nil {
					err = failure.Wrap("github.database.issue.comment.insert", err)
					logger.Debug().
						Err(err).
						Int("status", http.StatusInternalServerError).
						Str("code", "issue_comment_insert_failed").
						Msg("skip issue comment")
					continue
				}
			}
		}
	default:
		err = failure.Invalid("issue_state", "unsupported state "+issue.State)
		err = failure.Wrap("github.http.issue.state.validate", err)
		logger = logger.With().Str("issue_name", issue.Title).Str("issue_id", *issue.FullDatabaseId).Logger()
		logger.Debug().
			Err(err).
			Int("status", http.StatusBadRequest).
			Str("code", "issue_state_invalid").
			Str("issue_state", issue.State).
			Msg("skip issue")
		return true
	}
	return false
}

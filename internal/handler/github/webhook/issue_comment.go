package webhook

import (
	"context"
	"errors"
	"net/http"
	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/lib"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

func (h *WebhookHandler) WebhookIssueComment(w http.ResponseWriter, r *http.Request, body []byte, deliveryID string) {
	logger := zerolog.Ctx(r.Context()).With().Str("event", "issue_comment").Logger()
	trace, ok := auth.GetTrace(r.Context())
	if !ok {
		logger.Error().
			Int("status", http.StatusInternalServerError).
			Str("cause", "missing_tracing_context").
			Msg("handler invoked without trace ID in context")

		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "An internal server error occurred",
			Details: map[string]string{
				"reason": "request context pipeline uninitialized",
			},
		})
		return
	}
	timeout, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var req GitHubIssueComment
	err := sonic.Unmarshal(body, &req)
	if err != nil {
		logger.Warn().
			Err(err).
			Int("status", http.StatusBadRequest).
			Str("cause", "failed_to_unmarshal").
			Msg("failed to unmarshal the given body inside the structure")
		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "Could not unmarshal the given body due to an error",
			TraceID: trace,
		})
		return
	}

	change := req.Changes
	var bodyFrom *string
	if change != nil {
		bodyFrom = &req.Changes.Body.From
	}

	repoIDStr := strconv.FormatInt(req.Repository.ID, 10)
	hookIDStr, err := h.RedisClient.Get(timeout, "oauth:github:webhook:hook_id:"+repoIDStr).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("failed to fetch the webhook id from redis due to timeout")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Timedout while trying to get the webhook if",
				TraceID: trace,
			})
			return
		}
		if errors.Is(err, redis.Nil) {
			logger.Warn().
				Err(err).
				Int("status", http.StatusNotFound).
				Str("cause", "missing_webhook_id").
				Int64("repo_id", req.Repository.ID).
				Str("full_name", req.Repository.FullName).
				Msg("missing webhook id inside redis")
			lib.Pretty(w, http.StatusNotFound, lib.Error{
				Code:    "NOT_FOUND",
				Message: "Failed due to missing webhook id",
				TraceID: trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Msg("unrecognized error while trying to check for the webhook id inside redis")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Failed due to unrecognized error while trying to check for the webhook existence inside redis",
			TraceID: trace,
		})
		return
	}

	if hookIDStr == "" {
		logger.Warn().
			Int("status", http.StatusBadRequest).
			Str("cause", "empty_webhook_id").
			Msg("failed due to empty webhook id")
		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "Empty webhook id, cannot continue",
			TraceID: trace,
		})
		return
	}

	if req.Issue.Title == "" {
		logger.Error().
			Int("status", http.StatusBadRequest).
			Str("cause", "missing_issue_title").
			Msg("the given payload's issue title is missing")

		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "Invalid payload, issue title is missing",
			TraceID: trace,
		})

		return
	}

	if req.Issue.State != "open" && req.Issue.State != "closed" {
		logger.Error().
			Int("status", http.StatusBadRequest).
			Str("cause", "invalid_issue_state").
			Str("given_state", req.Issue.State).
			Msg("the given payload's state is not 'closed' or 'open'")
		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "Invalid payload, state is malformed",
			TraceID: trace,
		})
		return
	}

	exists, err := h.Queries.IssueInfoExistsInsideTheGitHub(timeout, req.Issue.ID)
	if err != nil {
		if errors.Is(err, pgconn.ErrConnClosed) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("failed to check if the issue info exists inside the database due to timeout")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Timedout while checking if the issue info exists inside the database",
				TraceID: trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Msg("failed to check if the issue info exists inside the database")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Unrecognized error while trying to check for issue info existence",
			TraceID: trace,
		})
		return
	}

	if !exists {
		logger.Warn().
			Int("status", http.StatusNotFound).
			Str("cause", "missing_issue_info").
			Msg("the given comment doesn't have a matching issue inside the database")
		lib.Pretty(w, http.StatusNotFound, lib.Error{
			Code:    "NOT_FOUND",
			Message: "Missing the issue relating to the comment inside the database",
			Details: map[string]string{
				"reason": "missing issue",
			},
		})
		return
	}

	positiveReactions := req.Issue.Reactions.Hooray + req.Issue.Reactions.PositiveReaction + req.Issue.Reactions.Heart + req.Issue.Reactions.Laugh + req.Issue.Reactions.Rocket
	negativeReactions := req.Issue.Reactions.Confused + req.Issue.Reactions.NegativeReaction

	err = h.Queries.InsertGitHubIssueComment(timeout, db.InsertGitHubIssueCommentParams{
		IssueID:           req.Issue.ID,
		Action:            req.Action,
		ChangeFrom:        bodyFrom,
		CreatedAt:         pgtype.Timestamptz{Time: req.Comment.CreatedAt, Valid: true},
		UpdatedAt:         pgtype.Timestamptz{Time: req.Comment.UpdatedAt, Valid: true},
		AuthorAssociation: req.Comment.AuthorAssociation,
		CommentID:         req.Comment.ID,
		CommentorID:       req.Comment.User.ID,
		CommentorName:     req.Comment.User.Name,
		CommentorType:     req.Comment.User.Type,
		Body:              req.Comment.Body,
		PositiveReactions: int32(positiveReactions),
		NegativeReactions: int32(negativeReactions),
		TotalReactions:    int32(req.Comment.Reactions.TotalCount),
	})
	if err != nil {
		if errors.Is(err, pgconn.ErrConnClosed) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("failed to insert the github issue comment due to timeout")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Timedout while trying to insert the github issue comment",
				TraceID: trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Msg("failed to insert the github issue comment due to an unrecognized error")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Unrecognized error while trying to insert the github issue comment",
			TraceID: trace,
		})
		return
	}

	logger.Info().
		Str("cause", "success").
		Msg("successfully inserted the given webhook issue comment inside the database")

	lib.Pretty(w, http.StatusOK, nil)
}

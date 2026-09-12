package kitanai

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"prolepsis/internal/auth"
	"prolepsis/internal/lib"

	"github.com/rs/zerolog"
)

func (h *Handler) ExtractPdf(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context()).With().Str("handler", "ExtractPdf").Logger()
	// If above 25 megabytes, we throw a tantrum
	// It would look like 25 * 1024 * 1024 (which are 25 megabytes in bytes)
	const maxLimit = 25 << 20
	u, ok := auth.FetchContextInsideHandler(w, r)
	if !ok {
		return
	}

	err := r.ParseMultipartForm(maxLimit)
	if err != nil {
		logger.Warn().
			Err(err).
			Int("status", http.StatusBadRequest).
			Str("cause", "file_surpasses_limit").
			Msg("the given file is above 25 megabytes")
		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "The given file is above 25 megabytes",
			Details: map[string]string{
				"reason": "limit surpassed",
				"fix":    "compress the file to lower the size or throw away some of it's content",
			},
			TraceID: u.Trace,
		})
		return
	}

	// We require the form field key to be "file". It is the Frontend's job to name it "file".
	// For example, the Frontend HTML must look like: <input type="file" name="file" />
	// Because they use name="file", we use r.FormFile("file") to extract it from the HTTP request (we use r.Body when the request is data - as a json payload).
	file, header, err := r.FormFile("file")

	logger.Debug().
		Str("file_name", header.Filename).
		Int64("file_size", header.Size).
		Msg("successfuly wrote the information about the file")

	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			logger.Warn().
				Err(err).
				Int("status", http.StatusBadRequest).
				Str("cause", "missing_file").
				Msg("the given request doesn't include the file")
			lib.Pretty(w, http.StatusBadRequest, lib.Error{
				Code:    "BAD_REQUEST",
				Message: "Missing file in the request",
				Details: map[string]string{
					"reason": "missing file",
					"fix":    "include the file in the request",
				},
				TraceID: u.Trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized_file_error").
			Msg("unrecognized error appeared while trying to fetch the file from the request")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Unrecognized error while trying to fetch the file",
			Details: map[string]string{
				"reason": "unrecognized",
				"fix":    "try with another file",
			},
			TraceID: u.Trace,
		})
		return
	}

	readLimit := io.LimitReader(file, maxLimit)
	rawPdf, err := io.ReadAll(readLimit)
	if err != nil {
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "pdf_reading_failed").
			Msg("failed to read the content of the provided pdf")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Failed to read the given pdf",
			TraceID: u.Trace,
		})
		return
	}

	// Empty pdf thingy
	if len(rawPdf) == 0 {
		logger.Warn().
			Int("status", http.StatusBadRequest).
			Str("cause", "empty_pdf").
			Msg("the provided pdf is emtpy")
		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "The provided pdf is empty",
			Details: map[string]string{
				"reason": "empty pdf",
				"fix":    "try sending a non-empty pdf",
			},
			TraceID: u.Trace,
		})
		return
	}

	// Not a pdf - scary
	if !bytes.HasPrefix(rawPdf, []byte("%PDF")) {
		logger.Warn().
			Int("status", http.StatusBadRequest).
			Str("cause", "not_a_pdf").
			Msg("the provided file is not a pdf")
		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "The provided file is not a pdf",
			Details: map[string]string{
				"reason": "not a pdf",
				"fix":    "watch at the file you are about to send and be sure it has the extension '.pdf'",
			},
			TraceID: u.Trace,
		})
		return
	}

	// Still not a pdf - scary
	contentType := http.DetectContentType(rawPdf[:512])
	if contentType != "application/pdf" {
		logger.Warn().
			Int("status", http.StatusBadRequest).
			Str("cause", "not_a_pdf").
			Str("exact_reason", "missing the application/pdf header").
			Msg("the provided pdf is not a pdf")
		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "The provided file is not a pdf",
			Details: map[string]string{
				"reason": "not a pdf",
				"fix":    "watch at the file you are about to send and be sure it has the extension '.pdf'",
			},
			TraceID: u.Trace,
		})
		return
	}

	text, err := h.PdfClient.ExtractPdf(r.Context(), header.Filename, rawPdf)
	if err != nil {
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "failed_grpc").
			Msg("failed to send content via gRPC")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Failed to send content via gRPC",
			TraceID: u.Trace,
		})
		return
	}

	logger.Info().
		Int("status", http.StatusOK).
		Msg("successfully sent request to gRPC")

	lib.Pretty(w, http.StatusOK, map[string]string{
		"raw_pdf": text,
	})
}

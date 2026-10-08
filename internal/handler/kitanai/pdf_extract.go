package kitanai

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"prolepsis/internal/auth"
	"prolepsis/internal/errlog"
	"prolepsis/internal/lib"

	"github.com/rs/zerolog"
)

func (h *Handler) ExtractPdf(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context()).With().Str("op", "extract_pdf").Logger()
	// If above 25 megabytes, we throw a tantrum
	// It would look like 25 * 1024 * 1024 (which are 25 megabytes in bytes)
	const maxLimit = 25 << 20
	u, ok := auth.FetchContextInsideHandler(w, r)
	if !ok {
		return
	}

	err := r.ParseMultipartForm(maxLimit)
	if err != nil {
		errlog.ParsingError(logger, "file_size", w, u.Trace, err)
		return
	}

	// We require the form field key to be "file". It is the Frontend's job to name it "file".
	// For example, the Frontend HTML must look like: <input type="file" name="file" />
	// Because they use name="file", we use r.FormFile("file") to extract it from the HTTP request (we use r.Body when the request is data - as a json payload).
	file, header, err := r.FormFile("file")
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			errlog.EmptyValueError(logger, "file", w, u.Trace)
			return
		}
		errlog.UnexpectedError(logger, "file_fetch", w, u.Trace, err)
		return
	}
	defer func() {
		err := file.Close()
		if err != nil {
			errlog.UnexpectedError(logger, "file_close", w, u.Trace, err)
			return
		}
	}()

	logger.Debug().
		Str("file_name", header.Filename).
		Int64("file_size", header.Size).
		Msg("file information retrieved successfully")

	readLimit := io.LimitReader(file, maxLimit)
	rawPdf, err := io.ReadAll(readLimit)
	if err != nil {
		errlog.UnexpectedError(logger, "file_read", w, u.Trace, err)
		return
	}

	// Empty pdf thingy
	if len(rawPdf) == 0 {
		errlog.EmptyValueError(logger, "file_content", w, u.Trace)
		return
	}

	// Not a pdf - scary
	if !bytes.HasPrefix(rawPdf, []byte("%PDF")) {
		errlog.MalformedError(logger, "file_content", w, u.Trace)
		return
	}

	// Still not a pdf - scary
	contentType := http.DetectContentType(rawPdf[:512])
	if contentType != "application/pdf" {
		errlog.MalformedError(logger, "file_content_type", w, u.Trace)
		return
	}

	text, err := h.PdfClient.ExtractPdf(r.Context(), header.Filename, rawPdf)
	if err != nil {
		errlog.UnexpectedError(logger, "grpc_request", w, u.Trace, err)
		return
	}

	logger.Info().
		Int("status", http.StatusOK).
		Msg("successfully sent request to gRPC")

	lib.Pretty(w, http.StatusOK, map[string]string{
		"raw_pdf": text,
	})
}

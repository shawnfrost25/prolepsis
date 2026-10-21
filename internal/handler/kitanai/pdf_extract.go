package kitanai

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"prolepsis/internal/auth"
	"prolepsis/internal/errlog"
	"prolepsis/internal/failure"
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
		err = failure.Wrap("kitanai.http.file.form.parse", err)
		errlog.Http(logger, w, "file_form", failure.MalformedError, err, u.Trace)
		return
	}

	// We require the form field key to be "file". It is the Frontend's job to name it "file".
	// For example, the Frontend HTML must look like: <input type="file" name="file" />
	// Because they use name="file", we use r.FormFile("file") to extract it from the HTTP request (we use r.Body when the request is data - as a json payload).
	file, header, err := r.FormFile("file")
	if err != nil {
		err = failure.Wrap("kitanai.http.file.form.fetch", err)
		if errors.Is(err, http.ErrMissingFile) {
			errlog.Http(logger, w, "file", failure.NotFoundError, err, u.Trace)
			return
		}
		errlog.General(logger, w, "file_fetch", failure.InternalGeneralError, err, u.Trace)
		return
	}
	defer func() {
		err := file.Close()
		if err != nil {
			err = failure.Wrap("kitanai.http.file.close", err)
			errlog.General(logger, w, "file_close", failure.InternalGeneralError, err, u.Trace)
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
		err = failure.Wrap("kitanai.http.file.content.read", err)
		errlog.General(logger, w, "file_read", failure.InternalGeneralError, err, u.Trace)
		return
	}

	// Empty pdf thingy
	if len(rawPdf) == 0 {
		err = failure.EmptyValue("file_content")
		err = failure.Wrap("kitanai.http.file.content.existence_check", err)
		errlog.Http(logger, w, "file_content", failure.EmptyValueError, err, u.Trace)
		return
	}

	// Not a pdf - scary
	if !bytes.HasPrefix(rawPdf, []byte("%PDF")) {
		err = failure.Invalid("file_content", "missing PDF signature")
		err = failure.Wrap("kitanai.http.file.content.signature_check", err)
		errlog.Http(logger, w, "file_content", failure.InvalidError, err, u.Trace)
		return
	}

	// Still not a pdf - scary
	contentType := http.DetectContentType(rawPdf[:512])
	if contentType != "application/pdf" {
		err = failure.Invalid("file_content_type", "detected content type is "+contentType)
		err = failure.Wrap("kitanai.http.file.content.type_check", err)
		errlog.Http(logger, w, "file_content_type", failure.InvalidError, err, u.Trace)
		return
	}

	text, err := h.PdfClient.ExtractPdf(r.Context(), w, r, header.Filename, rawPdf)
	if err != nil {
		err = failure.Wrap("kitanai.grpc.pdf.extract", err)
		errlog.General(logger, w, "pdf_extract", failure.InternalGeneralError, err, u.Trace)
		return
	}

	logger.Info().
		Int("status", http.StatusOK).
		Msg("successfully sent request to gRPC")

	lib.Pretty(w, http.StatusOK, map[string]string{
		"raw_pdf": text,
	})
}

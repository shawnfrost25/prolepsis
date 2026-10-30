package kitanai

import "testing"

func TestPdfExtract(t *testing.T) {
	t.Skip(`
		Skipping test, PdfExtract function requires passing a fully functional attachemnt of type 'MIME'.
		Test is not skipped forever, later might create a mock pdf_extract that will simulate the action \
		while still sending data via gRPC,
	`)
}

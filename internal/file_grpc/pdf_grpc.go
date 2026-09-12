package filegrpc

import (
	"context"
	"prolepsis/internal/auth"
	"prolepsis/internal/pb/common"
	"prolepsis/internal/pb/pdf"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// We create a structure, so later we will give it a value (before declaring again)
type PdfClient struct {
	conn   *grpc.ClientConn
	client pdf.PdfServiceClient
}

// The given address here must match the port whre rust listens, because we send the request to that exact port (where rust listens)
func NewClient(addr string) (*PdfClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &PdfClient{
		conn:   conn,
		client: pdf.NewPdfServiceClient(conn),
	}, err
}

func (p *PdfClient) Close() error {
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}

func (p *PdfClient) ExtractPdf(ctx context.Context, pdf_title string, pdf_bytes []byte) (string, error) {
	u, err := auth.FetchContextOutsideHandler(ctx)
	if err != nil {
		return "", err
	}

	timeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	response, err := p.client.PdfRPC(timeout, &pdf.PdfRequest{
		Ctx: &common.RequestContext{
			UserId: u.ID.String(),
			UserIp: u.IP,
			Method: u.Method,
			Path:   u.Path,
			Role:   string(u.Role),
			Trace:  u.Trace,
		},
		PdfTitle: pdf_title,
		RawPdf:   pdf_bytes,
	})
	if err != nil {
		return "", err
	}

	return response.GetPdfContent(), nil
}

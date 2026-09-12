#![cfg(test)]

use crate::common::RequestContext;
use crate::{
    pdf::{pdf_service_server::PdfService, PdfRequest},
    pdf_extract::MyPdfService,
};
use anyhow::Result;
use tonic::Request;

fn get_data() -> RequestContext {
    RequestContext {
        user_id: "test-user-id".to_string(),
        user_ip: "127.0.0.1".to_string(),
        method: "POST".to_string(),
        path: "/test-path".to_string(),
        role: "user".to_string(),
        trace: "test-trace".to_string(),
    }
}

#[tokio::test]
async fn test_pdf_successfull() -> Result<()> {
    let ctx = get_data();

    let pdf_bytes = std::fs::read(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/src/grpc/test_healthy.pdf"
    ))?;

    let service = MyPdfService::default();

    let payload = PdfRequest {
        ctx: Some(ctx),
        pdf_title: "test.pdf".to_string(),
        raw_pdf: pdf_bytes,
    };

    let request = Request::new(payload);

    let response = service.pdf_rpc(request).await;

    assert!(response.is_ok());
    assert_eq!(response?.into_inner().pdf_title, "test.pdf");

    Ok(())
}

#[tokio::test]
async fn test_pdf_failure() -> Result<()> {
    let ctx = get_data();

    let pdf_bytes = std::fs::read(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/src/grpc/test_defected.pdf"
    ))?;

    let service = MyPdfService::default();

    let payload = PdfRequest {
        ctx: Some(ctx),
        pdf_title: "not-existent".to_string(),
        raw_pdf: pdf_bytes,
    };

    let request = Request::new(payload);

    let response = service.pdf_rpc(request).await;

    assert!(
        response.is_err(),
        "Expected pdf_rpc to fail on malformed PDF, but it succeeded"
    );

    Ok(())
}

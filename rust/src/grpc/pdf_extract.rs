use pdf_oxide::PdfDocument;
use std::panic::catch_unwind;
use tonic::{Request, Response, Status};
use tracing::{self, error, info, info_span};

use crate::pdf::pdf_service_server::PdfService;
use crate::pdf::{PdfRequest, PdfResponse};

#[derive(Default)]
pub struct MyPdfService;

#[tonic::async_trait]
impl PdfService for MyPdfService {
    async fn pdf_rpc(&self, request: Request<PdfRequest>) -> Result<Response<PdfResponse>, Status> {
        let req = request.into_inner();
        let pdf_title = req.pdf_title;
        let pdf_content = req.raw_pdf;
        let ctx = req.ctx.ok_or_else(|| {
            Status::invalid_argument("Empty context while expected to be fully intact")
        })?;

        // From now on, every logging will have this data attached to it
        let span = info_span!(
            "default data",
            user_id = %ctx.user_id,
            user_ip = %ctx.user_ip,
            user_path = %ctx.path,
            user_role = %ctx.role,
            method = %ctx.method,
            trace = %ctx.trace
        );

        let _guard = span.enter();

        let doc = match catch_unwind(|| PdfDocument::from_bytes(pdf_content)) {
            Ok(Ok(doc)) => {
                info!(
                    status = 200,
                    "successfully turned the pdf content (bytes) to a pdf type"
                );
                doc
            }
            Ok(Err(err)) => {
                error!(
                    error = %err,
                    status = 500,
                    cause = "failed_pdf_parsing",
                    "failed to turn the pdf content (bytes) to a pdf type"
                );
                return Err(Status::internal("couldn't parse the content".to_string()));
            }
            Err(_) => {
                error!(
                    status = 500,
                    cause = "panic_while_parsing_pdf",
                    "encountered a panic while trying to parse the pdf"
                );
                return Err(Status::internal(
                    "couldn't parse the content due to panic".to_string(),
                ));
            }
        };

        let doc_pages = match catch_unwind(|| doc.page_count()) {
            Ok(Ok(pages_count)) => {
                info!(
                    status = 200,
                    "successfully counted the pages of the provided pdf"
                );
                pages_count
            }
            Ok(Err(err)) => {
                error!(
                    error = %err,
                    status = 500,
                    cause = "failed_pdf_page_counting",
                    "failed to retrieve page count of the pdf"
                );
                // "Enumerate" - seems like a hard word to understand... I use it as the synonym of "count" (just to sound more professional)
                return Err(Status::internal(
                    "couldn't enumerate the page count".to_string(),
                ));
            }
            Err(_) => {
                error!(
                    status = 500,
                    cause = "panic_while_counting_pages",
                    "failed to retrieve page count of the pdf"
                );
                return Err(Status::internal(
                    "failed to enumerate pages due to panic".to_string(),
                ));
            }
        };

        let mut full_text = String::new();

        for index in 0..doc_pages {
            match catch_unwind(|| doc.extract_text(index)) {
                Ok(Ok(content)) => {
                    info!(status = 200, "successfully read the given pdf content");
                    full_text.push_str(&content);
                }
                Ok(Err(err)) => {
                    error!(
                        error = %err,
                        status = 500,
                        cause = "failed_to_push_pdf_content",
                        "couldn't push the read content to the variable"
                    );
                    return Err(Status::internal(
                        "couldn't push the read content".to_string(),
                    ));
                }
                Err(_) => {
                    error!(
                        status = 500,
                        cause = "panic_while_pushing_pdf_content",
                        "encountered a panic while trying to push the given content"
                    );
                    return Err(Status::internal(
                        "panic while trying to push the content".to_string(),
                    ));
                }
            }
        }

        let result = PdfResponse {
            pdf_title,
            pdf_content: full_text,
        };

        Ok(Response::new(result))
    }
}

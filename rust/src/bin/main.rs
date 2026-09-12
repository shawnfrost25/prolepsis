use anyhow::Result;
use prolepsis_sidecar::pdf::pdf_service_server::PdfServiceServer;
use prolepsis_sidecar::pdf_extract::MyPdfService;
use tonic::transport::Server;

#[tokio::main]
async fn main() -> Result<()> {
    // See? If you enter "main.go" - you'll notice that this address is set as the address to send the request from golang to this address (meaning we are sending it to Rust)
    let addr = "0.0.0.0:8081".parse()?;

    let pdf_service = MyPdfService::default();

    println!("gRPC Server running smoothly on {}", addr);

    // Now Rust is silently listening on 0.0.0.0:8081 - were Golang will throw at it the pdfs
    Server::builder()
        .add_service(PdfServiceServer::new(pdf_service))
        .serve(addr)
        .await?;

    Ok(())
}

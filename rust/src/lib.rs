#[path = "utils/logger.rs"]
pub mod logger;

#[path = "grpc/pdf_extract.rs"]
pub mod pdf_extract;

#[path = "grpc/pdf_extract_test.rs"]
pub mod pdf_extract_test;

pub mod pdf {
    tonic::include_proto!("pdf");
}

pub mod common {
    tonic::include_proto!("common");
}

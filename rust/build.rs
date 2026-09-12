use anyhow::Result;

fn main() -> Result<()> {
    print!("cargo:rerun-if-changes=../proto/");

    tonic_prost_build::configure()
        .build_server(true)
        // Here we precise the path of proto and everything
        // First arguments: The protobuf files
        // Second arguments: The protobuf folder
        .compile_protos(
            &["../proto/pdf_grpc.proto", "../proto/common.proto"],
            &["../proto"],
        )?;

    Ok(())
}

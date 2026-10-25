```mermaid
flowchart TB;
    Main("Go Main")
    API("Go API - chi Router")
    Middleware("Middleware")
    AuthMiddleware("Auth_Middleware")
    LoggingMiddleware("Logger_Middleware")
    Context1("----- Context ----- \nIdContext \nRoleContext \nTimeContext \nDurationContext")
    Context2("----- Context ----- \nIdContext \nRoleContext \nTimeContext \nDurationContext \nTraceContext")
    ZeroLogger("----- Logging Fields ----- \nuser_id \nuser_ip \nmethod \npath \nrole \nstarted_at \ntrace_id")
    Client(["Client"])
    Validation{"Is the file a valid PDF?"}
    Checks("----- Checks ----- \nMultipart form - max 25 MB \nFile field exists \nContent is not empty \nPDF signature \nDetected content type")
    ExtractCall("Send the PDF for extraction")
    Rust("Rust sidecar \nPDF text extraction")
    Wrap("----- Wrap ----- \nAdd context to error")
    Kind("----- Kind ----- \nMatch errors by kind")
    Errlog("Errlog")
    HTTPError{"Is it an HTTP-specific error?"}

    Main-.->|"Registers routes and starts server"|API
    Client<-->|"Uploads the PDF"|API
    API<-.->|"Execute function 'FetchContextInsideHandler'"|Middleware
    Middleware-->|"Execute Auth_Middleware"|AuthMiddleware
    AuthMiddleware-->|"Insert Context"|Context1-->LoggingMiddleware-->|"Insert Context"|Context2
    LoggingMiddleware-->|"Insert Logging"|ZeroLogger
    API-->Validation
    Validation-.->Checks
    Validation-->|"Yes"|ExtractCall
    ExtractCall<-->|"gRPC"|Rust
    ExtractCall-->|"Return extracted text"|API
    Validation-->|"No"|HTTPError
    HTTPError-->|"Yes"|Kind-->Wrap
    HTTPError-->|"No"|Wrap
    Wrap-->|"Add error and let Errlog match"|Errlog
    Errlog-->API

    style Client fill:#E8F1FF,stroke:#3B82F6,color:#1E3A8A,stroke-width:2px
    style Main fill:#DDF4F0,stroke:#0F766E,color:#134E4A,stroke-width:2px

    style API fill:#E8F5E9,stroke:#2E7D32,color:#14532D,stroke-width:2px
    style Middleware fill:#E8F5E9,stroke:#2E7D32,color:#14532D,stroke-width:2px
    style AuthMiddleware fill:#E8F5E9,stroke:#2E7D32,color:#14532D,stroke-width:2px
    style LoggingMiddleware fill:#E8F5E9,stroke:#2E7D32,color:#14532D,stroke-width:2px
    style ExtractCall fill:#E8F5E9,stroke:#2E7D32,color:#14532D,stroke-width:2px

    style Rust fill:#E0F2FE,stroke:#0369A1,color:#0C4A6E,stroke-width:2px

    style Wrap fill:#FEE2E2,stroke:#DC2626,color:#7F1D1D,stroke-width:2px
    style Kind fill:#FEE2E2,stroke:#DC2626,color:#7F1D1D,stroke-width:2px
    style Errlog fill:#FEE2E2,stroke:#DC2626,color:#7F1D1D,stroke-width:2px

    style Context1 fill:#F1F5F9,stroke:#64748B,color:#334155,stroke-width:2px
    style Context2 fill:#F1F5F9,stroke:#64748B,color:#334155,stroke-width:2px
    style ZeroLogger fill:#F1F5F9,stroke:#64748B,color:#334155,stroke-width:2px
    style Checks fill:#F1F5F9,stroke:#64748B,color:#334155,stroke-width:2px

    style Validation fill:#FEF3C7,stroke:#B45309,color:#78350F,stroke-width:2px
    style HTTPError fill:#FEF3C7,stroke:#B45309,color:#78350F,stroke-width:2px
```
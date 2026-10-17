# Prolepsis

> A high-performance developer data ETL pipeline and context engine designed to aggregate multi-platform user telemetry into PostgreSQL, providing rich, structured context for AI interactions.

---

## Disclaimer

**Note:** This project is under active development. Core features are being implemented and tested rapidly, so APIs and functionality may break between commits as the architecture evolves. Most of the test and functions are temporary—only when the README.md is fully updated everything will be considered "finale-finale."

---

## Core Purpose

**Prolepsis** is an active ETL (Extract, Transform, Load) backend engineered to continuously fetch, normalize, and store a user's digital footprint across multiple platforms. By centralizing this data inside a structured PostgreSQL database, the system serves as a rich context provider, giving AI models deep, accurate visibility into the user's background, repositories, coding activity, and professional history during conversations.

---

## Key Features & Modules

* **Multi-Platform ETL Ingestion:**
  - **GitHub Integration:** Robust OAuth2 authentication and webhook event handling tracking pushes, commits, pull requests, issues, issue comments, and releases.
  - **Platform Connectors:** Extensible pipeline modules for aggregating developer metrics across platforms like **LeetCode**, **StackOverflow**, **etc...**
* **Rust gRPC PDF Extraction Sidecar:**
  - High-performance document parsing service built in Rust (using `tonic` and `prost`) to extract structured text and metadata from PDF files (with OCR capabilities planned).
* **Kitanai API & User Management:**
  - Complete backend lifecycle management: user registration, login, profile queries, session validation, and account verification.
  - Secure automated background job queues (via `River`) handling scheduled data deletions and cleanup tasks.
* **Session & Token Management:**
  - High-speed session management and token validation powered by Redis.

---

## Tech Stack & Architecture

* **Backend Core:** Go (Golang) microservice architecture.
* **gRPC Sidecar:** Rust (`tonic`, `prost`) for heavy document/PDF processing.
* **Database & Migrations:** PostgreSQL managed with **Atlas** schema migrations and type-safe query generation via **sqlc** (`pgxpool`).
* **Caching & Jobs:** Redis for session caching and state management; River for background job processing.
* **Containerization:** Docker & Docker Compose for multi-container orchestration (`compose.yaml`).

---

## Future Ideas
* **Rust Sidecar OCR Fallback:** Integrating an OCR fallback mechanism into the Rust PDF extraction sidecar to seamlessly process documents whenever `pdf_oxide` fails.
* **Advanced Telemetry:** Implementing robust observability and monitoring pipelines utilizing OpenTelemetry, Grafana Alloy, Grafana dashboards, and planned Prometheus integration.
* **Cloud & Infrastructure as Code:** Provisioning and managing cloud architecture on AWS using Terraform and OpenTofu with HCL syntax, remote state locking, and strict cost guardrails.
* **Kubernetes Orchestration:** Deploying services on Kubernetes via k3d using Helm charts, health probes, resource limits, and automated rolling updates with fast rollback capabilities.
* **Production-Grade Testing:** Elevating the test suite to mimic real-world production environments using containerized dependencies (Testcontainers), chaos-replay verification streams, and rigorous race detection.
* **GitHub App Migration:** Migrating the webhook creation to GitHub App.

---
## Note on Development and AI usage
* **AI Usage:** AI was used exclusively for low-value, repetitive tasks—primarily boilerplate and test scaffolding (such as sqlc test.sql queries and Redis test fixtures like create_session.redis). These represent pure busywork not worth writing by hand. AI also assisted with occasional debugging sessions when root causes weren’t immediate, and with polishing code comments and documentation tone (this README being one example).
* **Author Implementation:** Every meaningful design decision, all core application logic, and virtually all actual source code were thought through and written by the author. The AI’s role was strictly limited to the minor, non-creative tasks listed above.

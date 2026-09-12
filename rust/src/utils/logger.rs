// Generated with AI assistance.
use tracing::Level;
use tracing_appender::rolling::{RollingFileAppender, Rotation};
use tracing_subscriber::{
    fmt::{self, time::ChronoUtc},
    layer::SubscriberExt,
    util::SubscriberInitExt,
    EnvFilter,
};

pub fn init(level: &str) {
    let max_level = match level.to_lowercase().as_str() {
        "trace" => Level::TRACE,
        "debug" => Level::DEBUG,
        "info" => Level::INFO,
        "warn" => Level::WARN,
        "error" => Level::ERROR,
        _ => Level::INFO,
    };

    let filter = EnvFilter::from_default_env().add_directive(max_level.into());

    let file_appender = RollingFileAppender::builder()
        .rotation(Rotation::DAILY)
        .max_log_files(3)
        .filename_prefix("mimi")
        .filename_suffix("log")
        .build("/var/log/mimi")
        .expect("Failed to initialize file appender");

    let (non_blocking_file, _guard) = tracing_appender::non_blocking(file_appender);

    if max_level <= Level::DEBUG {
        let stdout_layer = fmt::layer()
            .with_timer(ChronoUtc::rfc_3339())
            .with_target(true)
            .with_file(true)
            .with_line_number(true)
            .with_level(true);

        let file_layer = fmt::layer()
            .with_timer(ChronoUtc::rfc_3339())
            .with_target(true)
            .with_file(true)
            .with_line_number(true)
            .with_level(true)
            .with_ansi(false)
            .with_writer(non_blocking_file);

        tracing_subscriber::registry()
            .with(filter)
            .with(stdout_layer)
            .with(file_layer)
            .init();
    } else {
        let stdout_layer = fmt::layer()
            .with_timer(fmt::time::Uptime::default())
            .json()
            .flatten_event(true)
            .with_current_span(false)
            .with_span_list(false)
            .with_target(false)
            .with_file(false)
            .with_line_number(false)
            .with_ansi(false);

        let file_layer = fmt::layer()
            .with_timer(fmt::time::Uptime::default())
            .json()
            .flatten_event(true)
            .with_current_span(false)
            .with_span_list(false)
            .with_target(false)
            .with_file(false)
            .with_line_number(false)
            .with_ansi(false)
            .with_writer(non_blocking_file);

        tracing_subscriber::registry()
            .with(filter)
            .with(stdout_layer)
            .with(file_layer)
            .init();
    }
}

//! haft-embed — local embedding sidecar.
//!
//! Speaks newline-delimited JSON over stdio or a Unix socket so haft can stream
//! embedding requests without a vector DB or a network hop.
//! Protocol:
//!   - on stdio startup, or on each socket connection, emits one handshake line:
//!     {"ready":true,"model":..,"dim":N}
//!     (or {"ready":false,"error":..} + exit 1 on stdio if the model fails)
//!   - then, per input line: request  {"id":N,"task":"query|document|raw","texts":[..]}
//!                           response {"id":N,"vectors":[[f32..]..]}  | {"id":N,"error":..}
//!   - EOF on stdin -> exit 0.
//!
//! The sidecar is OPTIONAL: when absent, haft degrades to FTS5+PPR recall.
//! It exists only to AUGMENT that recall with semantic similarity — the
//! decision graph stays primary (dec-20260605-fe77b358).

use std::io::{self, BufRead, BufReader, Write};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::{Duration, Instant};

#[cfg(unix)]
use std::os::unix::fs::PermissionsExt;
#[cfg(unix)]
use std::os::unix::net::{UnixListener, UnixStream};

use anyhow::{anyhow, Context, Result};
use clap::Parser;
use fastembed::{EmbeddingModel, InitOptions, TextEmbedding};
use ort::ep;
use serde::{Deserialize, Serialize};

/// Local embedding sidecar for haft.
#[derive(Parser, Debug)]
#[command(name = "haft-embed", version, about)]
struct Args {
    /// Embedding model id (embeddinggemma-300m | bge-small-en-v1.5).
    #[arg(long, default_value = "embeddinggemma-300m")]
    model: String,

    /// Directory where the ONNX model is cached/downloaded (haft passes ~/.haft/models).
    #[arg(long)]
    cache_dir: Option<PathBuf>,

    /// Matryoshka (MRL) truncation target dimension (768/512/256/128). 0 = native.
    #[arg(long, default_value_t = 0)]
    dim: usize,

    /// Show model-download progress on stderr (kept off so stdout stays a clean JSON stream).
    #[arg(long)]
    show_progress: bool,

    /// Serve the same JSON protocol over this Unix socket instead of stdio.
    #[arg(long)]
    serve_socket: Option<PathBuf>,

    /// Exit socket server mode after this many idle seconds. 0 = never exit.
    #[arg(long, default_value_t = 1200)]
    idle_timeout_secs: u64,

    /// Keep ONNX Runtime's CPU memory arena enabled.
    #[arg(long)]
    cpu_arena: bool,
}

#[derive(Deserialize)]
struct Request {
    id: u64,
    #[serde(default)]
    task: Task,
    texts: Vec<String>,
}

/// Retrieval role. EmbeddingGemma is asymmetric: queries and documents get
/// distinct canonical prompt prefixes (Google's spec). `raw` opts out.
#[derive(Deserialize, Default, Clone, Copy, PartialEq)]
#[serde(rename_all = "lowercase")]
enum Task {
    #[default]
    Query,
    Document,
    Raw,
}

#[derive(Clone, Serialize)]
struct Handshake {
    ready: bool,
    model: String,
    dim: usize,
}

#[derive(Serialize)]
struct Response {
    id: u64,
    #[serde(skip_serializing_if = "Option::is_none")]
    vectors: Option<Vec<Vec<f32>>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    error: Option<String>,
}

// ---- functional core: pure transforms, no IO ----

fn resolve_model(name: &str) -> Result<EmbeddingModel> {
    match name.to_ascii_lowercase().replace('_', "-").as_str() {
        "embeddinggemma-300m" | "embeddinggemma" | "gemma" => {
            Ok(EmbeddingModel::EmbeddingGemma300M)
        }
        // Quantized EmbeddingGemma: same 768-native MRL model, int8/4-bit weights —
        // 3-4x faster CPU inference and a far smaller first-use download than fp32,
        // with near-identical retrieval quality. Same asymmetric prefixes apply.
        "embeddinggemma-300m-q" | "gemma-q" => Ok(EmbeddingModel::EmbeddingGemma300MQ),
        "embeddinggemma-300m-q4" | "gemma-q4" => Ok(EmbeddingModel::EmbeddingGemma300MQ4),
        "bge-small-en-v1.5" | "bge-small" => Ok(EmbeddingModel::BGESmallENV15),
        other => Err(anyhow!("unknown model id: {other}")),
    }
}

/// Apply EmbeddingGemma's canonical asymmetric prompt prefix for the role.
fn prefix(task: Task, text: &str) -> String {
    match task {
        Task::Query => format!("task: search result | query: {text}"),
        Task::Document => format!("title: none | text: {text}"),
        Task::Raw => text.to_string(),
    }
}

/// MRL-truncate to `dim` (Matryoshka prefix dims are valid for EmbeddingGemma),
/// then L2-normalize so downstream cosine is a plain dot product.
fn truncate_normalize(mut vector: Vec<f32>, dim: usize) -> Vec<f32> {
    if dim > 0 && dim < vector.len() {
        vector.truncate(dim);
    }
    let norm: f32 = vector.iter().map(|value| value * value).sum::<f32>().sqrt();
    if norm > 0.0 {
        vector.iter_mut().for_each(|value| *value /= norm);
    }
    vector
}

// ---- effect boundary: the model handle ----

struct Embedder {
    model: TextEmbedding,
    dim: usize,
}

impl Embedder {
    fn load(args: &Args) -> Result<Self> {
        let kind = resolve_model(&args.model)?;
        let cpu_provider = ep::CPU::default()
            .with_arena_allocator(args.cpu_arena)
            .build();
        let mut options = InitOptions::new(kind)
            .with_show_download_progress(args.show_progress)
            .with_execution_providers(vec![cpu_provider]);
        if let Some(dir) = &args.cache_dir {
            options = options.with_cache_dir(dir.clone());
        }
        let mut model = TextEmbedding::try_new(options).context("load embedding model")?;

        let probe = model.embed(vec!["x"], None).context("probe embedding")?;
        let native = probe.first().map(Vec::len).unwrap_or(0);
        let dim = if args.dim > 0 && args.dim < native {
            args.dim
        } else {
            native
        };
        Ok(Self { model, dim })
    }

    fn embed(&mut self, task: Task, texts: &[String]) -> Result<Vec<Vec<f32>>> {
        let prepared: Vec<String> = texts.iter().map(|text| prefix(task, text)).collect();
        let vectors = self
            .model
            .embed(prepared, None)
            .context("embed texts")?
            .into_iter()
            .map(|vector| truncate_normalize(vector, self.dim))
            .collect();
        Ok(vectors)
    }
}

fn handle_line(embedder: &mut Embedder, line: &str) -> Response {
    let request: Request = match serde_json::from_str(line) {
        Ok(request) => request,
        Err(err) => {
            return Response {
                id: 0,
                vectors: None,
                error: Some(format!("bad request json: {err}")),
            }
        }
    };
    match embedder.embed(request.task, &request.texts) {
        Ok(vectors) => Response {
            id: request.id,
            vectors: Some(vectors),
            error: None,
        },
        Err(err) => Response {
            id: request.id,
            vectors: None,
            error: Some(format!("{err:#}")),
        },
    }
}

// ---- imperative shell ----

fn main() -> Result<()> {
    let args = Args::parse();
    if let Some(socket) = &args.serve_socket {
        return run_socket(&args, socket);
    }
    run_stdio(&args)
}

fn run_stdio(args: &Args) -> Result<()> {
    let stdout = io::stdout();
    let mut out = stdout.lock();

    let mut embedder = match Embedder::load(&args) {
        Ok(embedder) => embedder,
        Err(err) => {
            let line = serde_json::json!({ "ready": false, "error": format!("{err:#}") });
            writeln!(out, "{line}")?;
            out.flush()?;
            std::process::exit(1);
        }
    };

    let handshake = Handshake {
        ready: true,
        model: args.model.clone(),
        dim: embedder.dim,
    };
    writeln!(out, "{}", serde_json::to_string(&handshake)?)?;
    out.flush()?;

    let stdin = io::stdin();
    for line in stdin.lock().lines() {
        let line = line?;
        if line.trim().is_empty() {
            continue;
        }
        let response = handle_line(&mut embedder, &line);
        writeln!(out, "{}", serde_json::to_string(&response)?)?;
        out.flush()?;
    }
    Ok(())
}

#[cfg(unix)]
fn run_socket(args: &Args, socket: &Path) -> Result<()> {
    let embedder = Embedder::load(args).context("load embedding daemon")?;
    let handshake = Handshake {
        ready: true,
        model: args.model.clone(),
        dim: embedder.dim,
    };

    if socket.exists() {
        std::fs::remove_file(socket)
            .with_context(|| format!("remove stale socket {}", socket.display()))?;
    }
    let listener =
        UnixListener::bind(socket).with_context(|| format!("bind socket {}", socket.display()))?;
    std::fs::set_permissions(socket, std::fs::Permissions::from_mode(0o600))
        .with_context(|| format!("secure socket {}", socket.display()))?;
    listener
        .set_nonblocking(true)
        .context("set socket nonblocking")?;

    let embedder = Arc::new(Mutex::new(embedder));
    let active_requests = Arc::new(AtomicUsize::new(0));
    let last_activity = Arc::new(Mutex::new(Instant::now()));
    let idle_timeout = Duration::from_secs(args.idle_timeout_secs);

    loop {
        match listener.accept() {
            Ok((stream, _addr)) => {
                stream
                    .set_nonblocking(false)
                    .context("set client socket blocking")?;
                mark_activity(&last_activity);
                spawn_client(
                    stream,
                    embedder.clone(),
                    handshake.clone(),
                    active_requests.clone(),
                    last_activity.clone(),
                );
            }
            Err(err) if err.kind() == io::ErrorKind::WouldBlock => {
                if should_exit_idle(&active_requests, &last_activity, idle_timeout) {
                    break;
                }
                thread::sleep(Duration::from_millis(100));
            }
            Err(err) => return Err(err).context("accept socket client"),
        }
    }

    let _ = std::fs::remove_file(socket);
    Ok(())
}

#[cfg(not(unix))]
fn run_socket(_args: &Args, _socket: &Path) -> Result<()> {
    Err(anyhow!(
        "socket server mode is only supported on Unix platforms"
    ))
}

#[cfg(unix)]
fn spawn_client(
    stream: UnixStream,
    embedder: Arc<Mutex<Embedder>>,
    handshake: Handshake,
    active_requests: Arc<AtomicUsize>,
    last_activity: Arc<Mutex<Instant>>,
) {
    thread::spawn(move || {
        if let Err(err) = handle_client(stream, embedder, handshake, active_requests, last_activity)
        {
            eprintln!("haft-embed client error: {err:#}");
        }
    });
}

#[cfg(unix)]
struct ActiveRequest {
    active: Arc<AtomicUsize>,
    last_activity: Arc<Mutex<Instant>>,
}

#[cfg(unix)]
impl ActiveRequest {
    fn begin(active: Arc<AtomicUsize>, last_activity: Arc<Mutex<Instant>>) -> Self {
        active.fetch_add(1, Ordering::SeqCst);
        Self {
            active,
            last_activity,
        }
    }
}

#[cfg(unix)]
impl Drop for ActiveRequest {
    fn drop(&mut self) {
        self.active.fetch_sub(1, Ordering::SeqCst);
        mark_activity(&self.last_activity);
    }
}

#[cfg(unix)]
fn handle_client(
    stream: UnixStream,
    embedder: Arc<Mutex<Embedder>>,
    handshake: Handshake,
    active_requests: Arc<AtomicUsize>,
    last_activity: Arc<Mutex<Instant>>,
) -> Result<()> {
    let reader_stream = stream.try_clone().context("clone socket stream")?;
    let mut reader = BufReader::new(reader_stream);
    let mut out = stream;

    writeln!(out, "{}", serde_json::to_string(&handshake)?)?;
    out.flush()?;

    loop {
        let mut line = String::new();
        let bytes = reader.read_line(&mut line).context("read socket request")?;
        if bytes == 0 {
            return Ok(());
        }
        if line.trim().is_empty() {
            continue;
        }
        mark_activity(&last_activity);
        let _active_request = ActiveRequest::begin(active_requests.clone(), last_activity.clone());
        let response = {
            let mut guard = embedder
                .lock()
                .map_err(|_| anyhow!("embedding model lock poisoned"))?;
            handle_line(&mut guard, &line)
        };
        writeln!(out, "{}", serde_json::to_string(&response)?)?;
        out.flush()?;
    }
}

#[cfg(unix)]
fn mark_activity(last_activity: &Arc<Mutex<Instant>>) {
    if let Ok(mut last) = last_activity.lock() {
        *last = Instant::now();
    }
}

#[cfg(unix)]
fn should_exit_idle(
    active_requests: &Arc<AtomicUsize>,
    last_activity: &Arc<Mutex<Instant>>,
    idle_timeout: Duration,
) -> bool {
    if idle_timeout.is_zero() || active_requests.load(Ordering::SeqCst) > 0 {
        return false;
    }
    last_activity
        .lock()
        .map(|last| last.elapsed() >= idle_timeout)
        .unwrap_or(false)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn should_exit_idle_when_no_request_is_active() {
        let active_requests = Arc::new(AtomicUsize::new(0));
        let last_activity = Arc::new(Mutex::new(Instant::now() - Duration::from_secs(30)));

        assert!(should_exit_idle(
            &active_requests,
            &last_activity,
            Duration::from_secs(20),
        ));
    }

    #[test]
    fn should_not_exit_idle_while_request_is_active() {
        let active_requests = Arc::new(AtomicUsize::new(1));
        let last_activity = Arc::new(Mutex::new(Instant::now() - Duration::from_secs(30)));

        assert!(!should_exit_idle(
            &active_requests,
            &last_activity,
            Duration::from_secs(20),
        ));
    }
}

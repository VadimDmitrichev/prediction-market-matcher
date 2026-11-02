# Prediction Market Event Matcher

![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)
![gRPC](https://img.shields.io/badge/gRPC-Protobuf-244c5a?style=flat&logo=google)
![Architecture](https://img.shields.io/badge/Concurrency-Worker%20Pool-success)

A high-performance **gRPC microservice** written in Go designed to identify and match equivalent events and prediction markets across disparate platforms (e.g., Polymarket, Kalshi, PredictIt) where event titles and market questions are phrased differently.

---

## Key Engineering Highlights

* **Two-Pass Similarity Pipeline:** Avoids expensive false-positive matches by combining fast lexical filtering (rune-accurate Levenshtein distance + semantic inversion guards) with deep entity verification (proper noun extraction, sports/finance prefix normalization, and keyword overlap).
* **Semantic Inversion Protection:** Detects antonymous verbs and modal negations (`will` vs `won't`, `increased` vs `decreased`, `rose` vs `fell`) in $O(N)$ time using zero-allocation hash sets (`map[string]struct{}`) to prevent matching opposite market outcomes.
* **Bounded Worker Pool & OOM Protection:** Evaluates Cartesian products ($N \times M$ event groups) concurrently across configurable worker pools using bounded job/result channels (`workerCount * 2`), keeping memory footprint at $O(1)$ regardless of input payload size.
* **Full Context Propagation & Graceful Shutdown:** Propagates `context.Context` across all gRPC handlers, job dispatchers, and worker goroutines to halt CPU-intensive calculations immediately upon client cancellation or deadline expiration. Handles `SIGTERM`/`SIGINT` signals cleanly via `grpcServer.GracefulStop()`.
* **Hot-Loop Allocation Optimization:** Pre-compiles regular expressions and utilizes deterministic single-pass `strings.NewReplacer` instances at package initialization to minimize GC pressure under high RPS.

---

## 🏗️ Architecture & Matching Pipeline

```text
                       +----------------------------+
                       |   gRPC Client Request      |
                       |  (First & Second Markets)  |
                       +-------------+--------------+
                                     |
                                     v
                       +----------------------------+
                       |  Bounded Job Dispatcher    |
                       |   (Context-Aware Queue)    |
                       +-------------+--------------+
                                     |
            +------------------------+------------------------+
            |                        |                        |
            v                        v                        v
    +---------------+        +---------------+        +---------------+
    |   Worker #1   |        |   Worker #2   |  ...   |   Worker #N   |
    +-------+-------+        +-------+-------+        +-------+-------+
            |                        |                        |
            +------------------------+------------------------+
                                     |
                                     v
             +-----------------------------------------------+
             |          Two-Pass Similarity Engine           |
             |                                               |
             |  [Pass 1.1] Critical Verb & Negation Guard    |
             |  [Pass 1.2] Rune-Based Levenshtein Threshold  |
             |  [Pass 2.0] Proper Noun & Entity Verification |
             +-----------------------+-----------------------+
                                     |
                                     v
                       +----------------------------+
                       |  CompareMarketsResponse    |
                       +----------------------------+

```

---

## Performance & Benchmarks

The similarity engine has been profiled and benchmarked using native Go benchmarking tools (`go test -bench=. -benchmem`).

Below are the single-core throughput and memory allocation metrics measured on an AMD Ryzen 5 processor:

```bash
$ make bench
go test -bench=. -benchmem ./similarity
BenchmarkTwoPassSimilarityCheck-12    17937    67756 ns/op    4676 B/op    50 allocs/op
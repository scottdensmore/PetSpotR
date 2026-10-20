# PetSpotR

## 🚀 Go, Google Cloud Platform & Ollama Gemma 4

PetSpotR is an event-driven AI application for reporting and matching lost pets.
It runs natively on **Google Cloud Platform (GCP)** and **Locally** using
the **Go 1.26.5 toolchain**, **Ollama**, and **Gemma 4** vision models!

![Logo](./img/logo.svg)

- **Go Event-Driven Microservices**: Clean 100% pure Go microservice architecture
  (`lostpet-service`, `foundpet-service`, `pet-matcher`,
  `notification-service`, `web-frontend`).
- **Local Dev & Local AI**: Zero-dependency local orchestration using
  `docker-compose` and local Ollama (`gemma4:e2b`).
- **GCP Cloud Infrastructure**: Deployed on Google Cloud Run, Cloud Pub/Sub,
  Cloud Firestore, and GCS using OpenTofu
  ([`infra/opentofu/`](./infra/opentofu)).

---

## Featured Technologies

- [Go](https://go.dev) - High-performance backend programming language
- [Ollama](https://ollama.com) &
  [Gemma 4](https://huggingface.co/google/gemma-4-2b) - Local & Cloud AI vision
  model inference
- [Google Cloud Run](https://cloud.google.com/run) - Serverless container hosting
- [Google Cloud Pub/Sub](https://cloud.google.com/pubsub) - Scalable event-driven messaging
- [Google Cloud Firestore](https://cloud.google.com/firestore) - NoSQL document database
- [Google Cloud Storage](https://cloud.google.com/storage) - Object storage for pet images
- [OpenTofu](https://opentofu.org) / [Terraform](https://www.terraform.io) - Infrastructure as Code

---

## Getting Started

### Local Development

To run all unit tests and static analysis:

```bash
go vet ./...
go test -race -v -cover ./...
```

To start the local service containers and Ollama:

```bash
docker compose up --build
```

Compose waits for Ollama to become healthy and downloads `gemma4:e2b` into a
persistent volume before starting the pet matcher. Subsequent starts reuse the
downloaded model. Ollama stays inside the Compose network to avoid conflicting
with a host installation. Set `OLLAMA_MODEL` to override the default model.
The default stack uses process-local state and messaging, with optional
Dockerized Firestore and Pub/Sub emulators for durable, cross-process
integration testing.


## Build/test CI

The read-only Go workflow selects an already installed runner toolchain, verifies the checked-in module graph, runs vet, compiles every package, and runs the complete default Go test suite with race detection and coverage. It does not invoke Makefile tool fallbacks or install a Go toolchain. Application dependencies remain at their checked-in versions; existing dependency PRs are separate work. Gofmt output differs between Go 1.26 and 1.27 for existing multi-value returns, so this workflow adds no formatting gate or source churn while the separate toolchain PR remains unresolved.

Default tests use in-memory stores/brokers and local HTTP fakes, including the in-process event cascade. Existing opt-in Firestore/PubSub/Auth emulator, live GCS and live Ollama tests keep their original guards and are reported as unexecuted. CI supplies no cloud/provider credentials. Browser journeys, emulator processes, real model inference, infrastructure validation and golangci-lint are not claimed by this workflow. No Docker services, model downloads, deployment or publisher are invoked.

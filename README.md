# PetSpotR

## 🚀 Go, Google Cloud Platform & Ollama Gemma 4

PetSpotR is an event-driven AI application for reporting and matching lost pets.
It runs natively on **Google Cloud Platform (GCP)** and **Locally** using
**Go 1.22+**, **Ollama**, and **Gemma 4** vision models!

![Logo](./img/logo.svg)

- **Go Event-Driven Microservices**: Clean microservice architecture
  (`lostpet-service`, `foundpet-service`, `pet-matcher`,
  `notification-service`).
- **Local Dev & Local AI**: Zero-dependency local orchestration using
  `docker-compose` and local Ollama (`gemma4:2b`).
- **GCP Cloud Infrastructure**: Deployed on Google Cloud Run, Cloud Pub/Sub,
  Cloud Firestore, and GCS using OpenTofu
  ([`infra/opentofu/`](./infra/opentofu)).
- **Documentation**:
  - Setup & Deployment Guide: [`docs/DEVELOPMENT.md`](./docs/DEVELOPMENT.md)
  - Full Architecture & Migration Plan: [`docs/MIGRATION_PLAN.md`](./docs/MIGRATION_PLAN.md)
  - Coding Agent Guidelines: [`AGENTS.md`](./AGENTS.md)

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

To start the full local microservice stack and Ollama:

```bash
ollama pull gemma4:2b
docker-compose up --build
```

For complete development details and GCP deployment instructions, see
[`docs/DEVELOPMENT.md`](./docs/DEVELOPMENT.md).

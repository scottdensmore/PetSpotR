# PetSpotR

## 如果你需要中文请访问 [中文版本](./README_CN.md)

## 🚀 Go, Google Cloud & Ollama Gemma 2 Migration

PetSpotR has been modernized to run natively in **Google Cloud Platform (GCP)** and **Locally** using **Go 1.22+**, **Ollama**, and **Gemma 2** vision models!

- **Go Architecture**: Event-driven microservices (`lostpet-service`, `foundpet-service`, `pet-matcher`, `notification-service`).
- **Local Dev & Local AI**: Zero-dependency local orchestration using `docker-compose` and local Ollama (`gemma2:2b`).
- **GCP Cloud Infrastructure**: Deployed on Google Cloud Run, Cloud Pub/Sub, Cloud Firestore, and GCS using OpenTofu (`infra/opentofu`).
- **Documentation**:
  - Detailed Setup & Deployment Guide: [`docs/DEVELOPMENT.md`](file:///home/scottdensmore/Developer/scottdensmore/petspotr/docs/DEVELOPMENT.md)
  - Full Modernization Plan & Roadmap: [`docs/MIGRATION_PLAN.md`](file:///home/scottdensmore/Developer/scottdensmore/petspotr/docs/MIGRATION_PLAN.md)
  - Coding Agent Guidelines: [`Agents.md`](file:///home/scottdensmore/Developer/scottdensmore/petspotr/Agents.md)

---

## 🛠️ Here for a workshop? Go to the [workshop](workshop/README.md) folder to get started! 🛠️

---

PetSpotR allows you to use advanced AI models to report and find lost pets. It is a sample application that uses Azure Machine Learning to train a model to detect pets in images.

It also leverages popular open-source projects such as Dapr and Keda to provide a scalable and resilient architecture.

![Logo](./img/logo.svg)

## Featured technologies

- [Go](https://go.dev) - High-performance backend programming language
- [Ollama](https://ollama.com) & [Gemma 2](https://huggingface.co/google/gemma-2-2b) - Local & Cloud AI vision model inference
- [Google Cloud Run](https://cloud.google.com/run) - Serverless container hosting
- [OpenTofu](https://opentofu.org) / [Terraform](https://www.terraform.io) - Infrastructure as code
- [Bicep](https://docs.microsoft.com/en-us/azure/azure-resource-manager/bicep/overview) - Legacy Azure Infrastructure as code
- [Azure Kubernetes Service](https://docs.microsoft.com/en-us/azure/aks/intro-kubernetes)
- [Azure Blob Storage](https://docs.microsoft.com/en-us/azure/storage/blobs/storage-blobs-introduction)
- [Azure Cosmos DB](https://docs.microsoft.com/en-us/azure/cosmos-db/introduction)
- [Azure Service Bus](https://docs.microsoft.com/en-us/azure/service-bus-messaging/service-bus-messaging-overview)
- [Dapr](https://dapr.io) - Microservice building blocks

## Architecture

> **Note**: This application is a demo app which is not intended to be used in production. Use at your own risk.

### Go & GCP Architecture

![Go Architecture](./img/logo.svg)

For the step-by-step development, testing, and deployment commands, refer to [`docs/DEVELOPMENT.md`](file:///home/scottdensmore/Developer/scottdensmore/petspotr/docs/DEVELOPMENT.md).

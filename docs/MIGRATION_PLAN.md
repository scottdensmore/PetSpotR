# PetSpotR Migration Plan: Go, GCP, and Ollama/Gemma Integration

This document outlines the architecture, roadmap, and milestone tracking for
converting PetSpotR from Python/Azure/Blazor to **Go**, **Google Cloud
Platform (GCP)**, and **Ollama with Gemma 2 models**.

---

## 🎯 Strategic Goals

1. **Language Migration**: Convert backend (Flask) and frontend (.NET Blazor) to
   modular Go services (`cmd/backend`, `cmd/frontend` / unified web app).
2. **AI Provider Modernization**: Replace Azure Machine Learning with **Ollama**
   running locally and in GCP, leveraging **Gemma 2** (e.g. `gemma2:2b`,
   `gemma2:9b`) for pet identification, vision classification, and image score
   matching.
3. **Cloud Infrastructure**: Transition from Azure (AKS, Bicep, Cosmos DB,
   Service Bus, Azure Blob Storage) to **Google Cloud Platform** (Cloud Run,
   GCS, Cloud Pub/Sub, Cloud Firestore, Terraform/OpenTofu).
4. **Hybrid Runtime Capabilities**: Full parity for running locally (Docker
   Compose, Dapr, local Ollama) and in the cloud (GCP Cloud Run, managed GCP
   services).
5. **Agentic Workflow Adherence**: Strict compliance with
   [`Agents.md`](../Agents.md) (TDD, thin vertical slices, `ui-review`,
   `verifier`, and `code-review` subagents before commits).

---

## 🏗️ Architecture Comparison

| Component | Legacy (Python/Azure) | New (Go/GCP/Ollama) |
| :--- | :--- | :--- |
| **Backend** | Python 3.8 / Flask (`src/backend`) | Go 1.22+ (`cmd/backend`) |
| **Frontend** | .NET Blazor (`src/frontend`) | Go Web UI (`cmd/frontend`) |
| **AI / Scoring** | Azure ML Compute | Ollama + Gemma 2 (`gemma2:2b`/`9b`) |
| **Hosting** | Azure Kubernetes Service (AKS) | GCP Cloud Run / GKE |
| **Storage** | Azure Blob Storage | Google Cloud Storage (GCS) |
| **State** | Azure Cosmos DB / Redis | Google Cloud Firestore / MemoryStore |
| **Messaging** | Azure Service Bus / Redis PubSub | Google Cloud Pub/Sub |
| **IaC** | Azure Bicep (`iac/*.bicep`) | Terraform / OpenTofu (`iac/gcp`) |
| **Local Runtime** | Dapr CLI + Docker | Docker Compose + Dapr + Ollama |

---

## 🗺️ Migration Roadmap & GitHub Issues

### Phase 1: Go Core & Domain Model (Milestone 1)

- [ ] **[#2](https://github.com/scottdensmore/PetSpotR/issues/2)**
  **Phase 1.1:** Define Go Module structure, domain models, and unit test
  suite
- [ ] **[#3](https://github.com/scottdensmore/PetSpotR/issues/3)**
  **Phase 1.2:** Implement state store and pub/sub abstraction interfaces in Go

### Phase 2: Ollama & Gemma Model Service (Milestone 2)

- [ ] **[#4](https://github.com/scottdensmore/PetSpotR/issues/4)**
  **Phase 2.1:** Implement Go Ollama API client module supporting Gemma 2
  models
- [ ] **[#15](https://github.com/scottdensmore/PetSpotR/issues/15)**
  **Phase 2.2a:** Gemma 2 prompt engineering & structured JSON response parser
- [ ] **[#16](https://github.com/scottdensmore/PetSpotR/issues/16)**
  **Phase 2.2b:** Pet similarity scoring engine with feature comparison &
  threshold evaluation
- [ ] **[#6](https://github.com/scottdensmore/PetSpotR/issues/6)**
  **Phase 2.3:** Implement Pet Match Notification & Alert Engine in Go

### Phase 3: Go Backend & Web Frontend (Milestone 3)

- [ ] **[#17](https://github.com/scottdensmore/PetSpotR/issues/17)**
  **Phase 3.1a:** Go HTTP server bootstrap & /lostPet event handler
- [ ] **[#18](https://github.com/scottdensmore/PetSpotR/issues/18)**
  **Phase 3.1b:** /foundPet event handler with scoring integration & CloudEvent
  response
- [ ] **[#19](https://github.com/scottdensmore/PetSpotR/issues/19)**
  **Phase 3.2a:** Go Web Server skeleton & responsive base layout
- [ ] **[#20](https://github.com/scottdensmore/PetSpotR/issues/20)**
  **Phase 3.2b:** Lost Pet Report submission form page with image upload & preview
- [ ] **[#21](https://github.com/scottdensmore/PetSpotR/issues/21)**
  **Phase 3.2c:** Found Pet Search & Match results view with filter controls

### Phase 4: GCP Infrastructure & Local Dev Setup (Milestone 4)

- [ ] **[#22](https://github.com/scottdensmore/PetSpotR/issues/22)**
  **Phase 4.1a:** Terraform modules for GCP Storage (GCS) & Cloud Pub/Sub
- [ ] **[#23](https://github.com/scottdensmore/PetSpotR/issues/23)**
  **Phase 4.1b:** Terraform modules for GCP Cloud Run & Cloud Firestore
- [ ] **[#10](https://github.com/scottdensmore/PetSpotR/issues/10)**
  **Phase 4.2:** Configure Dapr GCP Component Definitions for Cloud Deployment
- [ ] **[#11](https://github.com/scottdensmore/PetSpotR/issues/11)**
  **Phase 4.3:** Create Local Development Setup with Docker Compose, Dapr, and
  Ollama/Gemma

### Phase 5: E2E Verification & CI/CD Pipeline (Milestone 5)

- [ ] **[#12](https://github.com/scottdensmore/PetSpotR/issues/12)**
  **Phase 5.1:** Migrate Playwright & End-to-End Test Suite for Go Web App
- [ ] **[#13](https://github.com/scottdensmore/PetSpotR/issues/13)**
  **Phase 5.2:** Implement GitHub Actions Workflow for Go CI/CD & GCP Cloud Run
  Deployment

---

## ⚡ Execution Guidelines for Coding Agents

When taking on any of the issues above, agents MUST follow the 11-step workflow
defined in [`Agents.md`](../Agents.md):

1. **Inspect before changing anything.**
2. **Create a branch first.** (e.g., `feat/issue-2-go-domain-models`)
3. **Choose a thin vertical slice.**
4. **Use Test-Driven Development (TDD)**: Write tests in `*_test.go` first.
5. **Inspect the complete diff and track out-of-scope discoveries.**
6. **Run `ui-review` before verification** (for UI changes).
7. **Run `verifier` before code review.**
8. **Run `code-review` before every commit.**
9. **Commit using Conventional Commits.**
10. **Create pull requests from the reviewed state.**
11. **Merge only clean, passing pull requests.**

# AI Inference Platform on Kubernetes

Demo infrastructure project showing how to build an AI inference platform using Kubernetes.

## Stack

- Kubernetes (k3s)
- vLLM for LLM inference
- Qdrant vector database
- Prometheus + Grafana observability
- Python API service

## Architecture

Users  
↓  
API service (Kubernetes)  
↓  
Vector search (Qdrant)  
↓  
LLM inference (vLLM GPU node)

## Goals

- demonstrate Kubernetes platform architecture
- integrate LLM inference
- implement observability
- build production-like infrastructure
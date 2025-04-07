# Ride Sharing sample k8s microservice project

## Services

- api-gateway: API Gateway
- driver-service: Driver Service

## Intro
This is a golang microservices sample project using Kubernetes for both local development and for production, making you more confident on developing new microservices and deploying them.

## Requirements
To run this project locally all you need is [Tilt](https://tilt.dev/) and [Minikube](https://minikube.sigs.k8s.io/docs/)

Additionally, the `/web` folder is a NextJS web app, for that you need NodeJS (v20.12.0).

The project also offers a `skaffold.yaml` file which is obsolete, it's still in the project for demo purposes of Tilt vs Skaffold. Use it if you know what you're doing.

## Dependencies Installation
The project uses Buf for Protocol Buffers and gRPC-Gateway for HTTP/JSON to gRPC translation. To install all required dependencies, run:

```bash
make install-deps
```

This will install:
- Buf CLI for Protocol Buffer management
- gRPC-Gateway dependencies
- Swagger UI for API documentation

## Run

```bash
tilt up
```
# API Gateway

A RESTful API Gateway for Microservices using gRPC-Gateway.

## Architecture

This API Gateway is built on the following principles:

1. **Clean Architecture**: The codebase is organized into layers:
   - Domain: Core business logic and interfaces
   - Application: Use cases and API handlers
   - Infrastructure: External dependencies and technical details

2. **SOLID Principles**:
   - Single Responsibility: Each component has one job
   - Open/Closed: Open for extension, closed for modification
   - Liskov Substitution: Interface implementations are interchangeable
   - Interface Segregation: Targeted, specific interfaces
   - Dependency Inversion: High-level modules don't depend on low-level modules

3. **gRPC-Gateway Integration**:
   - Automatic HTTP-to-gRPC translation
   - RESTful API endpoints from gRPC service definitions
   - Dynamic service discovery via Kubernetes
   - OpenAPI/Swagger documentation

## Features

- Automatic REST endpoint generation from gRPC services
- Kubernetes service discovery
- FastAPI-like HTTP handling (simple, declarative endpoints)
- OpenAPI/Swagger documentation
- Standardized error handling
- Authentication header forwarding
- Health checks

## Getting Started

### Prerequisites

- Go 1.19 or later
- Protocol Buffers compiler (protoc)
- Kubernetes cluster (or minikube for local development)

### Installation

1. Install dependencies:
   ```bash
   chmod +x services/api-gateway/scripts/install_dependencies.sh
   ./services/api-gateway/scripts/install_dependencies.sh
   ```

2. Update Buf modules:
   ```bash
   buf mod update
   ```

3. Generate protobuf files:
   ```bash
   ./services/api-gateway/scripts/generate_proto.sh
   ```

4. Update Go dependencies:
   ```bash
   ./services/api-gateway/scripts/update_deps.sh
   ```

### Configuration

The API Gateway is configured through environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| PORT | HTTP server port | 8080 |
| K8S_NAMESPACE | Kubernetes namespace for service discovery | ride-sharing |
| SERVICE_PREFIX | Prefix for service names to discover | user- |
| REFRESH_INTERVAL | Interval for refreshing service discovery | 3600s |
| SWAGGER_DIR | Directory for Swagger UI files | services/api-gateway/swagger |

### Running

```bash
go run services/api-gateway/cmd/main.go
```

## API Documentation

Once running, you can access the Swagger UI at:

```
http://localhost:8080/swagger/
```

## Service Integration

To add a new microservice to the gateway:

1. Define your gRPC service with HTTP annotations in a .proto file
2. Run the service update script:
   ```bash
   ./services/api-gateway/scripts/update_service.sh your-service-name
   ```
3. Update Buf modules and generate code:
   ```bash
   buf mod update
   ./services/api-gateway/scripts/generate_proto.sh
   ```

## Architecture Diagram

```
┌────────────────┐
│     Client     │
└───────┬────────┘
        │
        ▼
┌────────────────┐
│   API Gateway  │
│  gRPC-Gateway  │
└───────┬────────┘
        │
        ▼
┌────────────────┐
│ K8s Discovery  │
└───────┬────────┘
        │
        ▼
┌────────┴────────┐
│  Microservices  │
│  (gRPC servers) │
└─────────────────┘
```

## Development

Follow these best practices when developing:

1. Maintain clean separation of concerns between layers
2. Keep services focused on their core responsibilities
3. Use interfaces for dependency injection
4. Write tests for each layer
5. Document all public APIs
6. Use Buf for proto management (linting, breaking change detection, and code generation)

## License

This project is licensed under the MIT License - see the LICENSE file for details.
# Core Package

The `core` package provides the foundation for our microservices architecture, implementing clean architecture principles with a FastAPI-inspired approach to data transfer objects (DTOs).

## Directory Structure

```
pkg/core/
├── entity/      # Base entity definitions and interfaces
├── repository/  # Database and persistence abstractions  
├── usecase/     # Business logic and use case implementation
├── controller/  # HTTP and gRPC controllers
├── dto/         # DTO validation, mapping, and response utilities
├── types/       # Common types shared across packages
├── database/    # Database connection and migration utilities
├── logger/      # Logging utilities
└── server/      # HTTP and gRPC server implementations
```

## FastAPI-Inspired DTO Validation and Mapping

The core package provides a FastAPI-inspired approach to DTO validation and mapping using struct tags:

### 1. Declarative Validation with Struct Tags

Use struct tags for validation directly on your DTOs:

```go
type CreateUserDTO struct {
    Username string `json:"username" validate:"required"`
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
    Role     string `json:"role" validate:"oneof=admin manager officer"` // Using go-playground/validator tags
}
```

Supported validation rules (via `go-playground/validator`): `required`, `email`, `min`, `max`, `len`, `oneof`, `uuid`, `alphanum`, `numeric`, `url`, etc.

### 2. Automatic Validation in Use Cases

The `BaseUseCaseImpl` automatically validates incoming CreateDTOs and UpdateDTOs using `coreDTO.Validate`:

```go
// Example within BaseUseCaseImpl.Create method:
if err := coreDTO.Validate(dto); err != nil {
    // Handle validation errors (returns coreDTO.ValidationErrors)
}
```

### 3. Automatic Mapping in Use Cases

The `BaseUseCaseImpl` automatically maps between DTOs and entity pointers using `coreDTO.MapToEntity`:

```go
// Example within BaseUseCaseImpl.Create method:
var entityPtr T
if err := coreDTO.MapToEntity(dto, &entityPtr); err != nil {
    // Handle mapping error
}

// Example within BaseUseCaseImpl.Update method:
if err := coreDTO.MapToEntity(dto, existingEntityPtr); err != nil {
    // Handle mapping error
}
```

Mapping from entities to response DTOs often happens in the Controller layer or a dedicated presentation layer using `coreDTO.MapEntityToDTO`:

```go
// Example in a Controller:
var response UserResponseDTO
if err := coreDTO.MapEntityToDTO(entityPtr, &response); err != nil {
    // Handle mapping error
}
```

### 4. Simplified Use Case Creation

Since validation and basic mapping are handled internally by `BaseUseCaseImpl` using the `core/dto` utilities, you no longer need to pass separate mapper or validator instances when creating a base use case:

```go
// Create the base use case (no mapper/validator args needed)
baseUseCase := usecase.NewBaseUseCase(repo, logger)

// Service setup
func NewUserService(repo repository.BaseRepository[User], logger logger.Logger) *UserService {
    baseUseCase := usecase.NewBaseUseCase(repo, logger)
    return &UserService{
        BaseUseCaseImpl: baseUseCase,
    }
}
```

This approach centralizes the core validation/mapping logic, making service implementations cleaner and more focused on specific business rules.

## Example Usage

See the `services/user-service` (if available) for a practical implementation demonstrating these patterns. 
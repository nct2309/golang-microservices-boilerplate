package core

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ControllerMapper defines interface for mapping between requests, responses and entities
type ControllerMapper[Req any, Resp any, E Entity, CreateDTO any, UpdateDTO any] interface {
	RequestToCreateDTO(req interface{}) (CreateDTO, error)
	RequestToUpdateDTO(req interface{}) (UpdateDTO, error)
	EntityToResponse(entity *E) (Resp, error)
	EntitiesToListResponse(result *PaginationResult[E]) (interface{}, error)
	FilterOptionsFromRequest(req interface{}) FilterOptions
}

// BaseController provides common functionality for all gRPC controllers/handlers
// with generic type support for Request, Response and Entity
type BaseController[Req any, Resp any, E Entity, CreateDTO any, UpdateDTO any] struct {
	Logger  Logger
	UseCase BaseUseCase[E, CreateDTO, UpdateDTO]
	Mapper  ControllerMapper[Req, Resp, E, CreateDTO, UpdateDTO]
}

// NewBaseController creates a new instance of BaseController
func NewBaseController[Req any, Resp any, E Entity, CreateDTO any, UpdateDTO any](
	logger Logger,
	useCase BaseUseCase[E, CreateDTO, UpdateDTO],
	mapper ControllerMapper[Req, Resp, E, CreateDTO, UpdateDTO],
) *BaseController[Req, Resp, E, CreateDTO, UpdateDTO] {
	return &BaseController[Req, Resp, E, CreateDTO, UpdateDTO]{
		Logger:  logger,
		UseCase: useCase,
		Mapper:  mapper,
	}
}

// ExtractMetadata extracts a value from gRPC metadata
func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) ExtractMetadata(ctx context.Context, key string) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", errors.New("no metadata in context")
	}

	values := md.Get(key)
	if len(values) == 0 {
		return "", fmt.Errorf("metadata key %s not found", key)
	}

	return values[0], nil
}

// ExtractUUID extracts and parses a UUID from gRPC metadata
func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) ExtractUUID(ctx context.Context, key string) (uuid.UUID, error) {
	value, err := bc.ExtractMetadata(ctx, key)
	if err != nil {
		return uuid.Nil, err
	}

	id, err := uuid.Parse(value)
	if err != nil {
		bc.Logger.Error("Invalid UUID format", "value", value, "error", err)
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "invalid UUID format: %v", err)
	}

	return id, nil
}

// ValidateRequest performs validation on a request object
func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) ValidateRequest(req interface{}) error {
	// Check if request is nil for pointer types
	if reflect.ValueOf(req).Kind() == reflect.Ptr && reflect.ValueOf(req).IsNil() {
		return status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	// Check if the request implements a Validate() method
	if validator, ok := req.(interface{ Validate() error }); ok {
		if err := validator.Validate(); err != nil {
			bc.Logger.Error("Request validation failed", "error", err)
			return status.Errorf(codes.InvalidArgument, "validation error: %v", err)
		}
	}

	return nil
}

// HandleUseCaseError properly maps use case errors to gRPC status errors
func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) HandleUseCaseError(err error) error {
	// Check if it's our custom error type
	var useCaseError *UseCaseError
	format := "error: %v"
	if errors.As(err, &useCaseError) {
		switch useCaseError.Type {
		case ErrNotFound:
			return status.Errorf(codes.NotFound, format, useCaseError.Error())
		case ErrInvalidInput:
			return status.Errorf(codes.InvalidArgument, format, useCaseError.Error())
		case ErrUnauthorized:
			return status.Errorf(codes.Unauthenticated, format, useCaseError.Error())
		case ErrForbidden:
			return status.Errorf(codes.PermissionDenied, format, useCaseError.Error())
		case ErrConflict:
			return status.Errorf(codes.AlreadyExists, format, useCaseError.Error())
		case ErrInternal:
			bc.Logger.Error("Internal error", "error", useCaseError.Error())
			return status.Errorf(codes.Internal, format, "an internal error occurred")
		}
	}

	// Default case for unexpected errors
	bc.Logger.Error("Unhandled error", "error", err)
	return status.Errorf(codes.Internal, "an internal error occurred")
}

// LogRequest logs incoming gRPC requests
func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) LogRequest(ctx context.Context, method string, req interface{}) {
	// Extract request ID from metadata if available
	requestID, _ := bc.ExtractMetadata(ctx, "x-request-id")

	bc.Logger.Info("Received gRPC request",
		"method", method,
		"request_id", requestID,
	)
}

// LogResponse logs outgoing gRPC responses
func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) LogResponse(ctx context.Context, method string, resp interface{}, err error) {
	// Extract request ID from metadata if available
	requestID, _ := bc.ExtractMetadata(ctx, "x-request-id")

	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			bc.Logger.Error("gRPC request failed",
				"method", method,
				"request_id", requestID,
				"code", st.Code(),
				"message", st.Message(),
			)
		} else {
			bc.Logger.Error("gRPC request failed with non-status error",
				"method", method,
				"request_id", requestID,
				"error", err,
			)
		}
		return
	}

	bc.Logger.Info("gRPC request completed successfully",
		"method", method,
		"request_id", requestID,
	)
}

// Create handles creation of a new entity
func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) Create(ctx context.Context, req interface{}) (Resp, error) {
	var resp Resp

	// Log request
	bc.LogRequest(ctx, "Create", req)

	// Validate request
	if err := bc.ValidateRequest(req); err != nil {
		return resp, err
	}

	// Convert request to DTO
	createDTO, err := bc.Mapper.RequestToCreateDTO(req)
	if err != nil {
		bc.Logger.Error("Failed to convert request to DTO", "error", err)
		return resp, status.Errorf(codes.InvalidArgument, "invalid request data: %v", err)
	}

	// Create entity via use case
	entity, err := bc.UseCase.Create(ctx, createDTO)
	if err != nil {
		return resp, bc.HandleUseCaseError(err)
	}

	// Convert entity to response
	resp, err = bc.Mapper.EntityToResponse(entity)
	if err != nil {
		bc.Logger.Error("Failed to convert entity to response", "error", err)
		return resp, status.Errorf(codes.Internal, "failed to generate response")
	}

	// Log response
	bc.LogResponse(ctx, "Create", resp, nil)

	return resp, nil
}

// Get handles retrieval of an entity by ID
func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) Get(ctx context.Context, id uuid.UUID) (Resp, error) {
	var resp Resp

	// Log request
	bc.Logger.Info("Received Get request", "id", id)

	// Get entity via use case
	entity, err := bc.UseCase.GetByID(ctx, id)
	if err != nil {
		return resp, bc.HandleUseCaseError(err)
	}

	// Convert entity to response
	resp, err = bc.Mapper.EntityToResponse(entity)
	if err != nil {
		bc.Logger.Error("Failed to convert entity to response", "error", err)
		return resp, status.Errorf(codes.Internal, "failed to generate response")
	}

	return resp, nil
}

// List handles retrieval of entities with filtering and pagination
func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) List(ctx context.Context, req interface{}) (interface{}, error) {
	// Log request
	bc.Logger.Info("Received List request")

	// Extract filter options from request
	opts := bc.Mapper.FilterOptionsFromRequest(req)

	// List entities via use case
	result, err := bc.UseCase.List(ctx, opts)
	if err != nil {
		return nil, bc.HandleUseCaseError(err)
	}

	// Convert entities to response
	resp, err := bc.Mapper.EntitiesToListResponse(result)
	if err != nil {
		bc.Logger.Error("Failed to convert entities to list response", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to generate response")
	}

	return resp, nil
}

// Update handles updating an entity
func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) Update(ctx context.Context, id uuid.UUID, req interface{}) (Resp, error) {
	var resp Resp

	// Log request
	bc.LogRequest(ctx, "Update", req)

	// Validate request
	if err := bc.ValidateRequest(req); err != nil {
		return resp, err
	}

	// Convert request to DTO
	updateDTO, err := bc.Mapper.RequestToUpdateDTO(req)
	if err != nil {
		bc.Logger.Error("Failed to convert request to DTO", "error", err)
		return resp, status.Errorf(codes.InvalidArgument, "invalid request data: %v", err)
	}

	// Update entity via use case
	entity, err := bc.UseCase.Update(ctx, id, updateDTO)
	if err != nil {
		return resp, bc.HandleUseCaseError(err)
	}

	// Convert entity to response
	resp, err = bc.Mapper.EntityToResponse(entity)
	if err != nil {
		bc.Logger.Error("Failed to convert entity to response", "error", err)
		return resp, status.Errorf(codes.Internal, "failed to generate response")
	}

	// Log response
	bc.LogResponse(ctx, "Update", resp, nil)

	return resp, nil
}

// Delete handles deletion of an entity
func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) Delete(ctx context.Context, id uuid.UUID) error {
	// Log request
	bc.Logger.Info("Received Delete request", "id", id)

	// Delete entity via use case
	err := bc.UseCase.Delete(ctx, id)
	if err != nil {
		return bc.HandleUseCaseError(err)
	}

	return nil
}

// HardDelete handles permanent deletion of an entity
func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) HardDelete(ctx context.Context, id uuid.UUID) error {
	// Log request
	bc.Logger.Info("Received HardDelete request", "id", id)

	// Hard delete entity via use case
	err := bc.UseCase.HardDelete(ctx, id)
	if err != nil {
		return bc.HandleUseCaseError(err)
	}

	return nil
}

// --- Bulk Operation Handlers ---

// CreateMany handles creation of multiple entities
func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) CreateMany(ctx context.Context, dtos []CreateDTO) ([]*E, error) {
	// Log request
	bc.Logger.Info("Received CreateMany request", "count", len(dtos))

	// Create entities via use case
	entities, err := bc.UseCase.CreateMany(ctx, dtos)
	if err != nil {
		return nil, bc.HandleUseCaseError(err)
	}

	// Log response
	bc.Logger.Info("Successfully created multiple entities", "count", len(entities))

	return entities, nil
}

// UpdateMany handles updating multiple entities based on a filter
func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) UpdateMany(ctx context.Context, filter map[string]interface{}, dto UpdateDTO) (int64, error) {
	// Log request
	bc.Logger.Info("Received UpdateMany request", "filter_keys_count", len(filter))

	// Update entities via use case
	affected, err := bc.UseCase.UpdateMany(ctx, filter, dto)
	if err != nil {
		return 0, bc.HandleUseCaseError(err)
	}

	// Log response
	bc.Logger.Info("Successfully updated multiple entities", "affected_count", affected)

	return affected, nil
}

// DeleteMany handles soft deletion of multiple entities based on a filter
func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) DeleteMany(ctx context.Context, filter map[string]interface{}) (int64, error) {
	// Log request
	bc.Logger.Info("Received DeleteMany request", "filter_keys_count", len(filter))

	// Delete entities via use case
	affected, err := bc.UseCase.DeleteMany(ctx, filter)
	if err != nil {
		return 0, bc.HandleUseCaseError(err)
	}

	// Log response
	bc.Logger.Info("Successfully soft-deleted multiple entities", "affected_count", affected)

	return affected, nil
}

// HardDeleteMany handles permanent deletion of multiple entities based on a filter
func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) HardDeleteMany(ctx context.Context, filter map[string]interface{}) (int64, error) {
	// Log request
	bc.Logger.Info("Received HardDeleteMany request", "filter_keys_count", len(filter))

	// Hard delete entities via use case
	affected, err := bc.UseCase.HardDeleteMany(ctx, filter)
	if err != nil {
		return 0, bc.HandleUseCaseError(err)
	}

	// Log response
	bc.Logger.Info("Successfully hard-deleted multiple entities", "affected_count", affected)

	return affected, nil
}

// UnaryServerInterceptor returns a grpc.UnaryServerInterceptor that handles common functionality
func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Get request ID or generate a new one
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			md = metadata.MD{}
		}

		requestID := ""
		if ids := md.Get("x-request-id"); len(ids) > 0 {
			requestID = ids[0]
		} else {
			requestID = uuid.New().String()
			md = md.Copy()
			md.Set("x-request-id", requestID)
			ctx = metadata.NewIncomingContext(ctx, md)
		}

		// Log the incoming request
		bc.Logger.Info("Received gRPC request",
			"method", info.FullMethod,
			"request_id", requestID,
		)

		// Call the handler
		resp, err := handler(ctx, req)

		// Log the response
		if err != nil {
			st, ok := status.FromError(err)
			if ok {
				bc.Logger.Error("gRPC request failed",
					"method", info.FullMethod,
					"request_id", requestID,
					"code", st.Code(),
					"message", st.Message(),
				)
			} else {
				bc.Logger.Error("gRPC request failed with non-status error",
					"method", info.FullMethod,
					"request_id", requestID,
					"error", err,
				)
			}
		} else {
			bc.Logger.Info("gRPC request completed successfully",
				"method", info.FullMethod,
				"request_id", requestID,
			)
		}

		return resp, err
	}
}

// StreamServerInterceptor returns a grpc.StreamServerInterceptor that handles common functionality
func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// Get request ID or generate a new one
		ctx := ss.Context()
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			md = metadata.MD{}
		}

		requestID := ""
		if ids := md.Get("x-request-id"); len(ids) > 0 {
			requestID = ids[0]
		} else {
			requestID = uuid.New().String()
			md = md.Copy()
			md.Set("x-request-id", requestID)
			ctx = metadata.NewIncomingContext(ctx, md)
		}

		// Log the incoming stream request
		bc.Logger.Info("Received gRPC stream request",
			"method", info.FullMethod,
			"request_id", requestID,
		)

		// Create a wrapped stream that logs messages
		wrappedStream := &loggingServerStream{
			ServerStream: ss,
			logger:       bc.Logger,
			method:       info.FullMethod,
			requestID:    requestID,
			ctx:          ctx,
		}

		// Call the handler
		err := handler(srv, wrappedStream)

		// Log the stream completion
		if err != nil {
			st, ok := status.FromError(err)
			if ok {
				bc.Logger.Error("gRPC stream request failed",
					"method", info.FullMethod,
					"request_id", requestID,
					"code", st.Code(),
					"message", st.Message(),
				)
			} else {
				bc.Logger.Error("gRPC stream request failed with non-status error",
					"method", info.FullMethod,
					"request_id", requestID,
					"error", err,
				)
			}
		} else {
			bc.Logger.Info("gRPC stream request completed successfully",
				"method", info.FullMethod,
				"request_id", requestID,
			)
		}

		return err
	}
}

// loggingServerStream wraps grpc.ServerStream to provide logging capabilities
type loggingServerStream struct {
	grpc.ServerStream
	logger    Logger
	method    string
	requestID string
	ctx       context.Context
}

// Context returns the context with the request ID
func (s *loggingServerStream) Context() context.Context {
	return s.ctx
}

// RecvMsg intercepts message receiving to log and validate the message
func (s *loggingServerStream) RecvMsg(m interface{}) error {
	err := s.ServerStream.RecvMsg(m)
	if err != nil {
		return err
	}

	// Log received message
	s.logger.Debug("Received message from stream",
		"method", s.method,
		"request_id", s.requestID,
		"message_type", reflect.TypeOf(m).String(),
	)

	return nil
}

// SendMsg intercepts message sending to log the message
func (s *loggingServerStream) SendMsg(m interface{}) error {
	// Log sent message
	s.logger.Debug("Sending message to stream",
		"method", s.method,
		"request_id", s.requestID,
		"message_type", reflect.TypeOf(m).String(),
	)

	return s.ServerStream.SendMsg(m)
}

// DefaultControllerMapper provides a basic implementation of ControllerMapper
type DefaultControllerMapper[Req any, Resp any, E Entity, CreateDTO any, UpdateDTO any] struct{}

// RequestToCreateDTO converts a request to a create DTO
func (m *DefaultControllerMapper[Req, Resp, E, CreateDTO, UpdateDTO]) RequestToCreateDTO(req interface{}) (CreateDTO, error) {
	var dto CreateDTO

	// This is a basic implementation that assumes req can be directly converted to dto
	// In real applications, you would implement proper mapping logic
	if reflect.TypeOf(req) == reflect.TypeOf(dto) {
		return reflect.ValueOf(req).Interface().(CreateDTO), nil
	}

	return dto, fmt.Errorf("cannot convert request type %T to DTO type %T", req, dto)
}

// RequestToUpdateDTO converts a request to an update DTO
func (m *DefaultControllerMapper[Req, Resp, E, CreateDTO, UpdateDTO]) RequestToUpdateDTO(req interface{}) (UpdateDTO, error) {
	var dto UpdateDTO

	// This is a basic implementation that assumes req can be directly converted to dto
	// In real applications, you would implement proper mapping logic
	if reflect.TypeOf(req) == reflect.TypeOf(dto) {
		return reflect.ValueOf(req).Interface().(UpdateDTO), nil
	}

	return dto, fmt.Errorf("cannot convert request type %T to DTO type %T", req, dto)
}

// EntityToResponse converts an entity to a response
func (m *DefaultControllerMapper[Req, Resp, E, CreateDTO, UpdateDTO]) EntityToResponse(entity *E) (Resp, error) {
	var resp Resp

	// This is a basic implementation that assumes entity can be directly converted to resp
	// In real applications, you would implement proper mapping logic
	if reflect.TypeOf(*entity) == reflect.TypeOf(resp) {
		return reflect.ValueOf(*entity).Interface().(Resp), nil
	}

	// Try to copy matching fields
	entityValue := reflect.ValueOf(*entity)
	respValue := reflect.ValueOf(&resp).Elem()

	for i := 0; i < respValue.NumField(); i++ {
		respField := respValue.Type().Field(i)
		entityField := entityValue.FieldByName(respField.Name)

		if entityField.IsValid() && entityField.Type().AssignableTo(respField.Type) {
			respValue.Field(i).Set(entityField)
		}
	}

	return resp, nil
}

// EntitiesToListResponse converts a pagination result to a list response
func (m *DefaultControllerMapper[Req, Resp, E, CreateDTO, UpdateDTO]) EntitiesToListResponse(result *PaginationResult[E]) (interface{}, error) {
	// In real applications, you would implement proper mapping logic
	return result, nil
}

// FilterOptionsFromRequest extracts filter options from a request
func (m *DefaultControllerMapper[Req, Resp, E, CreateDTO, UpdateDTO]) FilterOptionsFromRequest(req interface{}) FilterOptions {
	// In real applications, you would implement proper extraction logic
	return DefaultFilterOptions()
}

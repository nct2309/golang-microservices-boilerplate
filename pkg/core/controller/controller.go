package controller

// import (
// 	"context"
// 	"errors"
// 	"fmt"
// 	"reflect"

// 	"github.com/google/uuid"
// 	"google.golang.org/grpc"
// 	"google.golang.org/grpc/codes"
// 	"google.golang.org/grpc/metadata"
// 	"google.golang.org/grpc/status"

// 	coreDTO "golang-microservices-boilerplate/pkg/core/dto" // Alias for sub-package if direct access needed
// 	"golang-microservices-boilerplate/pkg/core/entity"
// 	"golang-microservices-boilerplate/pkg/core/logger"  // Import logger
// 	"golang-microservices-boilerplate/pkg/core/types"   // Import types (FilterOptions, PaginationResult)
// 	"golang-microservices-boilerplate/pkg/core/usecase" // Import usecase (BaseUseCase, UseCaseError)
// )

// // BaseController provides common functionality for gRPC controllers.
// // It now relies on core validation/mapping via the embedded use case
// // and core wrapper functions for entity <-> response mapping.
// type BaseController[Req any, Resp any, E entity.Entity, CreateDTO any, UpdateDTO any] struct {
// 	Logger  logger.Logger                                // Use imported logger type
// 	UseCase usecase.BaseUseCase[E, CreateDTO, UpdateDTO] // Use imported usecase type
// 	// Mapper field removed
// }

// // NewBaseController creates a new instance of BaseController.
// func NewBaseController[Req any, Resp any, E entity.Entity, CreateDTO any, UpdateDTO any](
// 	logger logger.Logger,
// 	useCase usecase.BaseUseCase[E, CreateDTO, UpdateDTO],
// ) *BaseController[Req, Resp, E, CreateDTO, UpdateDTO] {
// 	return &BaseController[Req, Resp, E, CreateDTO, UpdateDTO]{
// 		Logger:  logger,
// 		UseCase: useCase,
// 	}
// }

// // ExtractMetadata remains the same.
// func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) ExtractMetadata(ctx context.Context, key string) (string, error) {
// 	md, ok := metadata.FromIncomingContext(ctx)
// 	if !ok {
// 		return "", errors.New("no metadata in context")
// 	}
// 	values := md.Get(key)
// 	if len(values) == 0 {
// 		return "", fmt.Errorf("metadata key %s not found", key)
// 	}
// 	return values[0], nil
// }

// // ExtractUUID remains the same.
// func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) ExtractUUID(ctx context.Context, key string) (uuid.UUID, error) {
// 	value, err := bc.ExtractMetadata(ctx, key)
// 	if err != nil {
// 		return uuid.Nil, err
// 	}
// 	id, err := uuid.Parse(value)
// 	if err != nil {
// 		bc.Logger.Error("Invalid UUID format", "value", value, "error", err)
// 		return uuid.Nil, status.Errorf(codes.InvalidArgument, "invalid UUID format: %v", err)
// 	}
// 	return id, nil
// }

// // ValidateRequest performs preliminary validation on a request object.
// // Use case layer handles the main DTO validation.
// func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) ValidateRequest(req interface{}) error {
// 	if req == nil {
// 		return status.Error(codes.InvalidArgument, "request cannot be nil")
// 	}
// 	v := reflect.ValueOf(req)
// 	if v.Kind() == reflect.Ptr && v.IsNil() {
// 		return status.Error(codes.InvalidArgument, "request pointer cannot be nil")
// 	}
// 	// Optional: Check for a Validate() method for controller-level checks if needed
// 	// if validator, ok := req.(interface{ Validate() error }); ok { ... }
// 	return nil
// }

// // HandleUseCaseError maps use case errors to gRPC status errors.
// func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) HandleUseCaseError(err error) error {
// 	var useCaseErr *usecase.UseCaseError // Use imported type
// 	format := "error: %v"
// 	if errors.As(err, &useCaseErr) {
// 		switch useCaseErr.Type {
// 		case usecase.ErrNotFound: // Use imported error types
// 			return status.Errorf(codes.NotFound, format, useCaseErr.Error())
// 		case usecase.ErrInvalidInput:
// 			// Check if the message contains structured validation errors
// 			// (This assumes ValidationErrors.Error() format)
// 			if details, ok := err.(coreDTO.ValidationErrors); ok {
// 				// Potentially add more details to the gRPC error
// 				return status.Errorf(codes.InvalidArgument, "validation failed: %s", details.Error())
// 			}
// 			return status.Errorf(codes.InvalidArgument, format, useCaseErr.Error())
// 		case usecase.ErrUnauthorized:
// 			return status.Errorf(codes.Unauthenticated, format, useCaseErr.Error())
// 		case usecase.ErrForbidden:
// 			return status.Errorf(codes.PermissionDenied, format, useCaseErr.Error())
// 		case usecase.ErrConflict:
// 			return status.Errorf(codes.AlreadyExists, format, useCaseErr.Error())
// 		case usecase.ErrInternal:
// 			bc.Logger.Error("Internal use case error", "error", useCaseErr.Error())
// 			return status.Errorf(codes.Internal, format, "an internal error occurred")
// 		}
// 	}
// 	// Default for unexpected errors
// 	bc.Logger.Error("Unhandled error in use case", "error", err)
// 	return status.Errorf(codes.Internal, "an unexpected internal error occurred")
// }

// // LogRequest remains the same.
// func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) LogRequest(ctx context.Context, method string, req interface{}) {
// 	requestID, _ := bc.ExtractMetadata(ctx, "x-request-id")
// 	bc.Logger.Info("Received gRPC request", "method", method, "request_id", requestID)
// }

// // LogResponse remains the same.
// func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) LogResponse(ctx context.Context, method string, resp interface{}, err error) {
// 	requestID, _ := bc.ExtractMetadata(ctx, "x-request-id")
// 	if err != nil {
// 		st, ok := status.FromError(err)
// 		if ok {
// 			bc.Logger.Error("gRPC request failed", "method", method, "request_id", requestID, "code", st.Code(), "message", st.Message())
// 		} else {
// 			bc.Logger.Error("gRPC request failed with non-status error", "method", method, "request_id", requestID, "error", err)
// 		}
// 		return
// 	}
// 	bc.Logger.Info("gRPC request completed successfully", "method", method, "request_id", requestID)
// }

// // --- CRUD Method Implementations (Updated) ---

// // Create handles creation of a new entity.
// func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) Create(ctx context.Context, req Req) (*Resp, error) {
// 	bc.LogRequest(ctx, "Create", req)

// 	if err := bc.ValidateRequest(req); err != nil {
// 		return nil, err
// 	}

// 	// Convert request (Req) to CreateDTO.
// 	// This often requires specific logic per controller or a helper.
// 	// Using MapToEntity assumes Req and CreateDTO have compatible structures.
// 	var createDTO CreateDTO
// 	if err := coreDTO.MapToEntity(req, &createDTO); err != nil { // Using MapToEntity for Req -> CreateDTO conversion
// 		bc.Logger.Error("Failed to map request to CreateDTO", "requestType", reflect.TypeOf(req), "dtoType", reflect.TypeOf(createDTO), "error", err)
// 		return nil, status.Errorf(codes.InvalidArgument, "invalid request format: %v", err)
// 	}

// 	// Use case handles validation and creation
// 	entityPtr, err := bc.UseCase.Create(ctx, createDTO)
// 	if err != nil {
// 		return nil, bc.HandleUseCaseError(err)
// 	}

// 	// Convert entity pointer (*E) to response (Resp).
// 	var resp Resp
// 	if err := coreDTO.MapToDTO(entityPtr, &resp); err != nil {
// 		bc.Logger.Error("Failed to map entity to response", "entityID", (*entityPtr).GetID(), "error", err)
// 		return nil, status.Errorf(codes.Internal, "failed to generate response")
// 	}

// 	bc.LogResponse(ctx, "Create", &resp, nil)
// 	return &resp, nil
// }

// // Get handles retrieval of an entity by ID.
// func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) Get(ctx context.Context, id uuid.UUID) (*Resp, error) {
// 	bc.LogRequest(ctx, "Get", map[string]interface{}{"id": id})

// 	entityPtr, err := bc.UseCase.GetByID(ctx, id)
// 	if err != nil {
// 		return nil, bc.HandleUseCaseError(err)
// 	}

// 	// Convert entity pointer (*E) to response (Resp).
// 	var resp Resp
// 	if err := coreDTO.MapToDTO(entityPtr, &resp); err != nil {
// 		bc.Logger.Error("Failed to map entity to response", "entityID", id, "error", err)
// 		return nil, status.Errorf(codes.Internal, "failed to generate response")
// 	}

// 	bc.LogResponse(ctx, "Get", &resp, nil)
// 	return &resp, nil
// }

// // List handles retrieval of entities with filtering and pagination.
// // The response type needs careful consideration - it might not be `Resp` directly,
// // but rather a specific list response structure (e.g., containing PaginationResult and items of type Resp).
// // For now, returning the raw PaginationResult. Adjust as needed.
// func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) List(ctx context.Context, req Req) (*types.PaginationResult[E], error) { // Returning PaginationResult directly for now
// 	bc.LogRequest(ctx, "List", req)

// 	// Extract filter options from request.
// 	// This likely needs custom logic per controller.
// 	opts := bc.extractFilterOptions(req) // Assume helper exists

// 	result, err := bc.UseCase.List(ctx, opts)
// 	if err != nil {
// 		return nil, bc.HandleUseCaseError(err)
// 	}

// 	// Here, we might need to map result.Items ([]*E) to a slice of Resp ([]Resp).
// 	// Returning the raw result for now.
// 	// TODO: Implement mapping from []*E to []Resp if needed for the specific controller's response structure.

// 	bc.LogResponse(ctx, "List", result, nil)
// 	return result, nil
// }

// // Helper placeholder for extracting filter options (needs implementation per controller)
// func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) extractFilterOptions(req Req) types.FilterOptions {
// 	// Basic placeholder: Attempts to map Req to FilterOptions.
// 	// Real implementation would likely parse query params or request body fields.
// 	opts := types.DefaultFilterOptions()
// 	_ = coreDTO.MapToEntity(req, &opts) // Try mapping, ignore error for placeholder
// 	return opts
// }

// // Update handles updating an entity.
// func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) Update(ctx context.Context, id uuid.UUID, req Req) (*Resp, error) {
// 	bc.LogRequest(ctx, "Update", req)

// 	if err := bc.ValidateRequest(req); err != nil {
// 		return nil, err
// 	}

// 	// Convert request (Req) to UpdateDTO.
// 	var updateDTO UpdateDTO
// 	if err := coreDTO.MapToEntity(req, &updateDTO); err != nil { // Using MapToEntity for Req -> UpdateDTO conversion
// 		bc.Logger.Error("Failed to map request to UpdateDTO", "requestType", reflect.TypeOf(req), "dtoType", reflect.TypeOf(updateDTO), "error", err)
// 		return nil, status.Errorf(codes.InvalidArgument, "invalid request format: %v", err)
// 	}

// 	// Use case handles validation and update
// 	entityPtr, err := bc.UseCase.Update(ctx, id, updateDTO)
// 	if err != nil {
// 		return nil, bc.HandleUseCaseError(err)
// 	}

// 	// Convert updated entity pointer (*E) to response (Resp).
// 	var resp Resp
// 	if err := coreDTO.MapToDTO(entityPtr, &resp); err != nil {
// 		bc.Logger.Error("Failed to map updated entity to response", "entityID", id, "error", err)
// 		return nil, status.Errorf(codes.Internal, "failed to generate response")
// 	}

// 	bc.LogResponse(ctx, "Update", &resp, nil)
// 	return &resp, nil
// }

// // Delete handles deletion of an entity.
// func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) Delete(ctx context.Context, id uuid.UUID) error {
// 	bc.LogRequest(ctx, "Delete", map[string]interface{}{"id": id})

// 	err := bc.UseCase.Delete(ctx, id)
// 	if err != nil {
// 		return bc.HandleUseCaseError(err)
// 	}

// 	bc.LogResponse(ctx, "Delete", nil, nil)
// 	return nil
// }

// // HardDelete handles permanent deletion of an entity.
// func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) HardDelete(ctx context.Context, id uuid.UUID) error {
// 	bc.LogRequest(ctx, "HardDelete", map[string]interface{}{"id": id})

// 	err := bc.UseCase.HardDelete(ctx, id)
// 	if err != nil {
// 		return bc.HandleUseCaseError(err)
// 	}

// 	bc.LogResponse(ctx, "HardDelete", nil, nil)
// 	return nil
// }

// // --- Bulk Operation Handlers (Updated - Response mapping might need adjustment) ---

// // CreateMany handles creation of multiple entities.
// // Returns slice of pointers to created entities (*E). Response mapping TBD.
// func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) CreateMany(ctx context.Context, dtos []CreateDTO) ([]*E, error) {
// 	bc.LogRequest(ctx, "CreateMany", map[string]interface{}{"count": len(dtos)})

// 	// Use case handles validation and creation
// 	entities, err := bc.UseCase.CreateMany(ctx, dtos)
// 	if err != nil {
// 		return nil, bc.HandleUseCaseError(err)
// 	}

// 	// TODO: Map []*E to a suitable response type if needed.
// 	// Returning []*E directly for now.
// 	bc.LogResponse(ctx, "CreateMany", map[string]interface{}{"created_count": len(entities)}, nil)
// 	return entities, nil
// }

// // UpdateMany handles updating multiple entities based on a filter.
// func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) UpdateMany(ctx context.Context, filter map[string]interface{}, dto UpdateDTO) (int64, error) {
// 	bc.LogRequest(ctx, "UpdateMany", map[string]interface{}{"filter_keys": len(filter)})

// 	// Use case handles validation and update
// 	affected, err := bc.UseCase.UpdateMany(ctx, filter, dto)
// 	if err != nil {
// 		return 0, bc.HandleUseCaseError(err)
// 	}

// 	bc.LogResponse(ctx, "UpdateMany", map[string]interface{}{"affected_count": affected}, nil)
// 	return affected, nil
// }

// // DeleteMany handles soft deletion of multiple entities based on a filter.
// func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) DeleteMany(ctx context.Context, filter map[string]interface{}) (int64, error) {
// 	bc.LogRequest(ctx, "DeleteMany", map[string]interface{}{"filter_keys": len(filter)})

// 	affected, err := bc.UseCase.DeleteMany(ctx, filter)
// 	if err != nil {
// 		return 0, bc.HandleUseCaseError(err)
// 	}

// 	bc.LogResponse(ctx, "DeleteMany", map[string]interface{}{"affected_count": affected}, nil)
// 	return affected, nil
// }

// // HardDeleteMany handles permanent deletion of multiple entities based on a filter.
// func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) HardDeleteMany(ctx context.Context, filter map[string]interface{}) (int64, error) {
// 	bc.LogRequest(ctx, "HardDeleteMany", map[string]interface{}{"filter_keys": len(filter)})

// 	affected, err := bc.UseCase.HardDeleteMany(ctx, filter)
// 	if err != nil {
// 		return 0, bc.HandleUseCaseError(err)
// 	}

// 	bc.LogResponse(ctx, "HardDeleteMany", map[string]interface{}{"affected_count": affected}, nil)
// 	return affected, nil
// }

// // --- Interceptors (Updated Logger Type) ---

// // UnaryServerInterceptor remains largely the same, uses injected logger.
// func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
// 	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
// 		md, ok := metadata.FromIncomingContext(ctx)
// 		if !ok {
// 			md = metadata.MD{}
// 		}
// 		requestID := ""
// 		if ids := md.Get("x-request-id"); len(ids) > 0 {
// 			requestID = ids[0]
// 		} else {
// 			requestID = uuid.New().String()
// 			md = md.Copy()
// 			md.Set("x-request-id", requestID)
// 			ctx = metadata.NewIncomingContext(ctx, md)
// 		}

// 		bc.Logger.Info("Received gRPC request", "method", info.FullMethod, "request_id", requestID)
// 		resp, err := handler(ctx, req)

// 		if err != nil {
// 			st, ok := status.FromError(err)
// 			if ok {
// 				bc.Logger.Error("gRPC request failed", "method", info.FullMethod, "request_id", requestID, "code", st.Code(), "message", st.Message())
// 			} else {
// 				bc.Logger.Error("gRPC request failed with non-status error", "method", info.FullMethod, "request_id", requestID, "error", err)
// 			}
// 		} else {
// 			bc.Logger.Info("gRPC request completed successfully", "method", info.FullMethod, "request_id", requestID)
// 		}
// 		return resp, err
// 	}
// }

// // StreamServerInterceptor remains largely the same, uses injected logger.
// func (bc *BaseController[Req, Resp, E, CreateDTO, UpdateDTO]) StreamServerInterceptor() grpc.StreamServerInterceptor {
// 	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
// 		ctx := ss.Context()
// 		md, ok := metadata.FromIncomingContext(ctx)
// 		if !ok {
// 			md = metadata.MD{}
// 		}
// 		requestID := ""
// 		if ids := md.Get("x-request-id"); len(ids) > 0 {
// 			requestID = ids[0]
// 		} else {
// 			requestID = uuid.New().String()
// 			md = md.Copy()
// 			md.Set("x-request-id", requestID)
// 			ctx = metadata.NewIncomingContext(ctx, md)
// 		}

// 		bc.Logger.Info("Received gRPC stream request", "method", info.FullMethod, "request_id", requestID)

// 		wrappedStream := &loggingServerStream{
// 			ServerStream: ss,
// 			logger:       bc.Logger,
// 			method:       info.FullMethod,
// 			requestID:    requestID,
// 			ctx:          ctx,
// 		}

// 		err := handler(srv, wrappedStream)

// 		if err != nil {
// 			st, ok := status.FromError(err)
// 			if ok {
// 				bc.Logger.Error("gRPC stream request failed", "method", info.FullMethod, "request_id", requestID, "code", st.Code(), "message", st.Message())
// 			} else {
// 				bc.Logger.Error("gRPC stream request failed with non-status error", "method", info.FullMethod, "request_id", requestID, "error", err)
// 			}
// 		} else {
// 			bc.Logger.Info("gRPC stream request completed successfully", "method", info.FullMethod, "request_id", requestID)
// 		}
// 		return err
// 	}
// }

// // loggingServerStream uses imported logger.Logger type.
// type loggingServerStream struct {
// 	grpc.ServerStream
// 	logger    logger.Logger // Use imported logger type
// 	method    string
// 	requestID string
// 	ctx       context.Context
// }

// // Context remains the same.
// func (s *loggingServerStream) Context() context.Context {
// 	return s.ctx
// }

// // RecvMsg remains the same.
// func (s *loggingServerStream) RecvMsg(m interface{}) error {
// 	err := s.ServerStream.RecvMsg(m)
// 	if err != nil {
// 		return err
// 	}
// 	s.logger.Debug("Received message from stream", "method", s.method, "request_id", s.requestID, "message_type", reflect.TypeOf(m).String())
// 	return nil
// }

// // SendMsg remains the same.
// func (s *loggingServerStream) SendMsg(m interface{}) error {
// 	s.logger.Debug("Sending message to stream", "method", s.method, "request_id", s.requestID, "message_type", reflect.TypeOf(m).String())
// 	return s.ServerStream.SendMsg(m)
// }

// // --- DefaultControllerMapper REMOVED ---
// // Specific controllers should handle their own request/response mapping if needed,
// // potentially using coreDTO.MapToEntity/coreDTO.MapEntityToDTO or custom logic.

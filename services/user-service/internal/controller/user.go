package controller

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	coreController "golang-microservices-boilerplate/pkg/core/controller"
	coreTypes "golang-microservices-boilerplate/pkg/core/types"
	corePb "golang-microservices-boilerplate/proto/core"
	pb "golang-microservices-boilerplate/proto/user-service"
	"golang-microservices-boilerplate/services/user-service/internal/entity"
	userservice_usecase "golang-microservices-boilerplate/services/user-service/internal/usecase"
)

// UserServer defines the interface for the gRPC service handler.
// This corresponds to the pb.UserServiceServer interface but allows for dependency injection.
type UserServer interface {
	pb.UserServiceServer // Embed the generated interface
	// Add any other methods specific to the server lifecycle if needed
}

// Ensure userServer implements UserServer interface (and pb.UserServiceServer).
var _ UserServer = (*userServer)(nil)

// --- gRPC Server Implementation ---

type userServer struct {
	pb.UnimplementedUserServiceServer
	uc     userservice_usecase.UserUsecase
	mapper Mapper // Use the Mapper interface
}

// NewUserServer creates a new gRPC server instance.
// Accepts Mapper interface and returns UserServer interface.
func NewUserServer(uc userservice_usecase.UserUsecase, mapper Mapper) UserServer {
	return &userServer{
		uc:     uc,
		mapper: mapper, // Inject mapper
	}
}

// RegisterUserServiceServer registers the user service implementation with the gRPC server.
// Accepts use case and mapper to create the server.
func RegisterUserServiceServer(s *grpc.Server, uc userservice_usecase.UserUsecase, mapper Mapper) {
	server := NewUserServer(uc, mapper) // Pass mapper
	pb.RegisterUserServiceServer(s, server)
}

// Create implements proto.UserServiceServer.
func (s *userServer) Create(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	// Map proto directly to entity
	userEntity, err := s.mapper.ProtoCreateToEntity(req)
	if err != nil {
		return nil, status.Errorf(http.StatusBadRequest, "failed to map request: %v", err)
	}

	// Call use case Create method with the entity
	err = s.uc.Create(ctx, userEntity)
	if err != nil {
		return nil, coreController.MapErrorToHttpStatus(err)
	}

	// The userEntity is updated in place (e.g., with ID) by the Create method
	userProto, err := s.mapper.EntityToProto(userEntity)
	if err != nil {
		return nil, status.Errorf(http.StatusInternalServerError, "failed to map result: %v", err)
	}

	return &pb.CreateUserResponse{User: userProto}, nil
}

// GetByID implements proto.UserServiceServer.
func (s *userServer) GetByID(ctx context.Context, req *pb.GetUserByIDRequest) (*pb.GetUserByIDResponse, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Errorf(http.StatusBadRequest, "invalid user ID format: %v", err)
	}

	user, err := s.uc.GetByID(ctx, id)
	if err != nil {
		return nil, coreController.MapErrorToHttpStatus(err)
	}

	userProto, err := s.mapper.EntityToProto(user)
	if err != nil {
		return nil, status.Errorf(http.StatusInternalServerError, "failed to map result: %v", err)
	}

	return &pb.GetUserByIDResponse{User: userProto}, nil
}

// List implements proto.UserServiceServer.
func (s *userServer) List(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	opts := s.mapper.ProtoListRequestToFilterOptions(req)

	result, err := s.uc.List(ctx, opts)
	if err != nil {
		return nil, coreController.MapErrorToHttpStatus(err)
	}

	response, err := s.mapper.PaginationResultToProtoList(result)
	if err != nil {
		return nil, status.Errorf(http.StatusInternalServerError, "failed to map result list: %v", err)
	}

	return response, nil
}

// Update implements proto.UserServiceServer.
func (s *userServer) Update(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UpdateUserResponse, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Errorf(http.StatusBadRequest, "invalid user ID format: %v", err)
	}

	// 1. Get the existing user entity
	existingUser, err := s.uc.GetByID(ctx, id)
	if err != nil {
		return nil, coreController.MapErrorToHttpStatus(err) // Handle not found etc.
	}

	// 2. Apply updates from proto request to the existing entity
	if err := s.mapper.ApplyProtoUpdateToEntity(req, existingUser); err != nil {
		return nil, status.Errorf(http.StatusBadRequest, "failed to map update request: %v", err)
	}

	// 3. Call the use case Update method with the modified entity
	err = s.uc.Update(ctx, existingUser)
	if err != nil {
		return nil, coreController.MapErrorToHttpStatus(err)
	}

	// 4. Map the updated entity back to proto for response
	userProto, err := s.mapper.EntityToProto(existingUser)
	if err != nil {
		return nil, status.Errorf(http.StatusInternalServerError, "failed to map result: %v", err)
	}

	return &pb.UpdateUserResponse{User: userProto}, nil
}

// Delete implements proto.UserServiceServer (handles soft and hard delete).
func (s *userServer) Delete(ctx context.Context, req *pb.DeleteUserRequest) (*emptypb.Empty, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Errorf(http.StatusBadRequest, "invalid user ID format: %v", err)
	}

	hardDelete := req.GetHardDelete() // Get the flag from the request

	// Call the consolidated use case method
	if err := s.uc.Delete(ctx, id, hardDelete); err != nil {
		return nil, coreController.MapErrorToHttpStatus(err)
	}

	return &emptypb.Empty{}, nil
}

// FindWithFilter implements proto.UserServiceServer.
func (s *userServer) FindWithFilter(ctx context.Context, req *pb.FindUsersWithFilterRequest) (*pb.FindUsersWithFilterResponse, error) {
	// Map the options from the request, which now contains the filters map internally
	opts := coreTypes.DefaultFilterOptions()
	if req.Options != nil {
		opts = s.mapper.ProtoListRequestToFilterOptions(&pb.ListUsersRequest{Options: req.Options})
	}

	// Pass opts.Filters directly to the use case
	result, err := s.uc.FindWithFilter(ctx, opts.Filters, opts)
	if err != nil {
		return nil, coreController.MapErrorToHttpStatus(err)
	}

	// Need to map PaginationResult[entity.User] to FindUsersWithFilterResponse
	usersProto := make([]*pb.User, 0, len(result.Items))
	for _, userEntity := range result.Items {
		userProto, mapErr := s.mapper.EntityToProto(userEntity)
		if mapErr != nil {
			return nil, status.Errorf(http.StatusInternalServerError, "failed to map user entity %s: %v", userEntity.ID, mapErr)
		}
		usersProto = append(usersProto, userProto)
	}

	paginationInfo := &corePb.PaginationInfo{ // Use corepb alias
		TotalItems: result.TotalItems,
		Limit:      int32(result.Limit),
		Offset:     int32(result.Offset),
	}

	return &pb.FindUsersWithFilterResponse{
		Users:          usersProto,
		PaginationInfo: paginationInfo,
	}, nil
}

// CreateMany implements proto.UserServiceServer.
func (s *userServer) CreateMany(ctx context.Context, req *pb.CreateUsersRequest) (*pb.CreateUsersResponse, error) {
	if req == nil || len(req.Users) == 0 {
		return &pb.CreateUsersResponse{Users: []*pb.User{}}, nil
	}

	// Map proto requests directly to entities
	entities := make([]*entity.User, 0, len(req.Users))
	for i, createReq := range req.Users {
		userEntity, err := s.mapper.ProtoCreateToEntity(createReq)
		if err != nil {
			return nil, status.Errorf(http.StatusBadRequest, "failed to map user %d in bulk request: %v", i, err)
		}
		entities = append(entities, userEntity)
	}

	// Call use case CreateMany, capturing the returned entities and error
	createdEntities, err := s.uc.CreateMany(ctx, entities)
	if err != nil {
		return nil, coreController.MapErrorToHttpStatus(err)
	}

	// Map the returned created entities (now with IDs) back to proto
	usersProto := make([]*pb.User, 0, len(createdEntities))
	for _, userEntity := range createdEntities { // Use the returned slice
		userProto, mapErr := s.mapper.EntityToProto(userEntity)
		if mapErr != nil {
			// Log or handle potential partial failure? For now, fail the whole request.
			return nil, status.Errorf(http.StatusInternalServerError, "failed to map created user %s: %v", userEntity.ID, mapErr)
		}
		usersProto = append(usersProto, userProto)
	}

	return &pb.CreateUsersResponse{Users: usersProto}, nil
}

// UpdateMany implements proto.UserServiceServer.
// Note: The proto currently defines the response as Empty.
// This implementation calls the usecase which returns updated entities, but discards them to match the proto.
func (s *userServer) UpdateMany(ctx context.Context, req *pb.UpdateUsersRequest) (*emptypb.Empty, error) {
	if req == nil || len(req.Items) == 0 {
		return &emptypb.Empty{}, nil // Nothing to update
	}

	// Map proto request items to the map expected by the use case
	// Instead of mapping to DTOs, fetch entities and apply updates
	entitiesToUpdate := make([]*entity.User, 0, len(req.Items))
	for i, item := range req.Items {
		id, err := uuid.Parse(item.GetId())
		if err != nil {
			return nil, status.Errorf(http.StatusBadRequest, "invalid user ID format for item %d: %v", i, err)
		}

		// Fetch existing entity
		existingUser, err := s.uc.GetByID(ctx, id)
		if err != nil {
			return nil, coreController.MapErrorToHttpStatus(err)
		}

		// Create a temporary UpdateUserRequest from the item to reuse mapping logic
		updateReq := &pb.UpdateUserRequest{
			Id:         item.GetId(),
			Username:   item.Username,
			Email:      item.Email,
			Password:   item.Password,
			FirstName:  item.FirstName,
			LastName:   item.LastName,
			Role:       item.Role,
			IsActive:   item.IsActive,
			Phone:      item.Phone,
			Address:    item.Address,
			Age:        item.Age,
			ProfilePic: item.ProfilePic,
		}
		if err := s.mapper.ApplyProtoUpdateToEntity(updateReq, existingUser); err != nil {
			return nil, status.Errorf(http.StatusBadRequest, "failed to map update item %d (ID: %s): %v", i, id, err)
		}

		entitiesToUpdate = append(entitiesToUpdate, existingUser)
	}

	// Call the use case UpdateMany, capturing the returned updated entities and error
	_, err := s.uc.UpdateMany(ctx, entitiesToUpdate) // Capture and discard the returned slice for now
	if err != nil {
		return nil, coreController.MapErrorToHttpStatus(err)
	}

	// Return empty response on success as defined by the current proto
	return &emptypb.Empty{}, nil
}

// DeleteMany implements proto.UserServiceServer (handles soft and hard delete).
func (s *userServer) DeleteMany(ctx context.Context, req *pb.DeleteUsersRequest) (*emptypb.Empty, error) {
	if req == nil || len(req.Ids) == 0 {
		return &emptypb.Empty{}, nil // Nothing to delete
	}

	hardDelete := req.GetHardDelete()
	uuidSlice := make([]uuid.UUID, 0, len(req.Ids))

	for i, idStr := range req.Ids {
		id, err := uuid.Parse(idStr)
		if err != nil {
			return nil, status.Errorf(http.StatusBadRequest, "invalid user ID format at index %d: %v", i, err)
		}
		uuidSlice = append(uuidSlice, id)
	}

	// Call the consolidated use case method
	if err := s.uc.DeleteMany(ctx, uuidSlice, hardDelete); err != nil {
		return nil, coreController.MapErrorToHttpStatus(err)
	}

	return &emptypb.Empty{}, nil
}

// Login implements proto.UserServiceServer.
func (s *userServer) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	// Map proto to schema.LoginCredentials
	creds, err := s.mapper.ProtoLoginToSchema(req)
	if err != nil {
		return nil, status.Errorf(http.StatusBadRequest, "failed to map login request: %v", err)
	}

	// Call use case Login, which now returns schema.LoginResult
	loginResult, err := s.uc.Login(ctx, creds)
	if err != nil {
		return nil, coreController.MapErrorToHttpStatus(err)
	}

	// Map the schema.LoginResult to proto response using the mapper
	response, err := s.mapper.SchemaLoginResultToProto(loginResult)
	if err != nil {
		return nil, status.Errorf(http.StatusInternalServerError, "failed to map login result: %v", err)
	}

	return response, nil
}

// Refresh implements proto.UserServiceServer.
func (s *userServer) Refresh(ctx context.Context, req *pb.RefreshRequest) (*pb.RefreshResponse, error) {
	refreshToken := req.GetRefreshToken()
	if refreshToken == "" {
		return nil, status.Errorf(http.StatusBadRequest, "refresh token cannot be empty")
	}

	// Call use case Refresh, returns schema.RefreshResult
	refreshResult, err := s.uc.Refresh(ctx, refreshToken)
	if err != nil {
		return nil, coreController.MapErrorToHttpStatus(err)
	}

	// Map the schema.RefreshResult to proto response using the mapper
	response, err := s.mapper.SchemaRefreshResultToProto(refreshResult)
	if err != nil {
		return nil, status.Errorf(http.StatusInternalServerError, "failed to map refresh result: %v", err)
	}

	return response, nil
}

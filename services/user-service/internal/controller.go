package internal

import (
	"context"
	"time"

	"golang-microservices-boilerplate/pkg/core"
	proto "golang-microservices-boilerplate/proto/user-service"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UserController implements the gRPC user service
type UserController struct {
	// Embedding BaseController provides common gRPC handling logic
	*core.BaseController[*proto.CreateUserRequest, *proto.UserResponse, User, CreateUserDTO, UpdateUserDTO]
	userUseCase UserUseCase
	mapper      *UserControllerMapper
	proto.UnimplementedUserServiceServer
}

// NewUserController creates a new UserController
func NewUserController(userUseCase UserUseCase, logger core.Logger) *UserController {
	mapper := &UserControllerMapper{}
	baseController := core.NewBaseController[*proto.CreateUserRequest](
		logger,
		userUseCase,
		mapper,
	)

	return &UserController{
		BaseController: baseController,
		userUseCase:    userUseCase,
		mapper:         mapper,
	}
}

// RegisterGrpcHandlers registers the controller with the gRPC server
func (c *UserController) RegisterGrpcHandlers(grpcServer *grpc.Server) {
	proto.RegisterUserServiceServer(grpcServer, c)
}

// CreateUser handles user creation
func (c *UserController) CreateUser(ctx context.Context, req *proto.CreateUserRequest) (*proto.UserResponse, error) {
	c.Logger.Info("CreateUser request received", "email", req.Email)

	// Convert request to DTO
	createDTO, err := c.mapper.RequestToCreateDTO(req)
	if err != nil {
		return nil, c.HandleUseCaseError(err)
	}

	// Create user
	user, err := c.userUseCase.Create(ctx, createDTO)
	if err != nil {
		return nil, c.HandleUseCaseError(err)
	}

	// Convert entity to response
	response, err := c.mapper.EntityToResponse(user)
	if err != nil {
		c.Logger.Error("Failed to convert entity to response", "error", err)
		return nil, status.Error(codes.Internal, "Failed to generate response")
	}

	c.Logger.Info("User created successfully", "id", user.ID.String())
	return response, nil
}

// GetUser handles user retrieval by ID
func (c *UserController) GetUser(ctx context.Context, req *proto.GetUserRequest) (*proto.UserResponse, error) {
	c.Logger.Info("GetUser request received", "id", req.Id)

	// Parse user ID
	userID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid user ID format")
	}

	// Get user
	user, err := c.userUseCase.GetByID(ctx, userID)
	if err != nil {
		return nil, c.HandleUseCaseError(err)
	}

	// Convert entity to response
	response, err := c.mapper.EntityToResponse(user)
	if err != nil {
		c.Logger.Error("Failed to convert entity to response", "error", err)
		return nil, status.Error(codes.Internal, "Failed to generate response")
	}

	return response, nil
}

// UpdateUser handles user update
func (c *UserController) UpdateUser(ctx context.Context, req *proto.UpdateUserRequest) (*proto.UserResponse, error) {
	c.Logger.Info("UpdateUser request received", "id", req.Id)

	// Parse user ID
	userID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid user ID format")
	}

	// Convert request to DTO
	updateDTO, err := c.mapper.RequestToUpdateDTO(req)
	if err != nil {
		return nil, c.HandleUseCaseError(err)
	}

	// Update user
	user, err := c.userUseCase.Update(ctx, userID, updateDTO)
	if err != nil {
		return nil, c.HandleUseCaseError(err)
	}

	// Convert entity to response
	response, err := c.mapper.EntityToResponse(user)
	if err != nil {
		c.Logger.Error("Failed to convert entity to response", "error", err)
		return nil, status.Error(codes.Internal, "Failed to generate response")
	}

	c.Logger.Info("User updated successfully", "id", user.ID.String())
	return response, nil
}

// DeleteUser handles user deletion
func (c *UserController) DeleteUser(ctx context.Context, req *proto.DeleteUserRequest) (*proto.DeleteUserResponse, error) {
	c.Logger.Info("DeleteUser request received", "id", req.Id)

	// Parse user ID
	userID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid user ID format")
	}

	// Delete user
	err = c.userUseCase.Delete(ctx, userID)
	if err != nil {
		return nil, c.HandleUseCaseError(err)
	}

	c.Logger.Info("User deleted successfully", "id", req.Id)
	return &proto.DeleteUserResponse{Success: true}, nil
}

// ListUsers handles listing users with pagination
func (c *UserController) ListUsers(ctx context.Context, req *proto.ListUsersRequest) (*proto.ListUsersResponse, error) {
	c.Logger.Info("ListUsers request received", "limit", req.Limit, "offset", req.Offset)

	// Get filter options from request
	opts := c.mapper.FilterOptionsFromRequest(req)

	// List users
	result, err := c.userUseCase.List(ctx, opts)
	if err != nil {
		return nil, c.HandleUseCaseError(err)
	}

	// Convert entities to response
	response, err := c.mapper.EntitiesToListResponse(result)
	if err != nil {
		c.Logger.Error("Failed to convert entities to list response", "error", err)
		return nil, status.Error(codes.Internal, "Failed to generate response")
	}

	c.Logger.Info("Listed users successfully", "count", len(result.Data))
	return response.(*proto.ListUsersResponse), nil
}

// Login handles user authentication
func (c *UserController) Login(ctx context.Context, req *proto.LoginRequest) (*proto.LoginResponse, error) {
	c.Logger.Info("Login request received", "email", req.Email)

	// Extract login parameters
	email, password, err := c.mapper.LoginRequestToParams(req)
	if err != nil {
		return nil, c.HandleUseCaseError(err)
	}

	// Authenticate user
	user, token, err := c.userUseCase.Login(ctx, email, password)
	if err != nil {
		return nil, c.HandleUseCaseError(err)
	}

	// Generate expiry time (24 hours from now)
	expiresAt := time.Now().Add(24 * time.Hour)

	// Convert user and token to response
	response, err := c.mapper.UserAndTokenToLoginResponse(user, token, expiresAt)
	if err != nil {
		c.Logger.Error("Failed to convert login response", "error", err)
		return nil, status.Error(codes.Internal, "Failed to generate response")
	}

	c.Logger.Info("User logged in successfully", "id", user.ID.String())
	return response, nil
}

// FindByEmail handles user lookup by email
func (c *UserController) FindByEmail(ctx context.Context, req *proto.FindByEmailRequest) (*proto.UserResponse, error) {
	c.Logger.Info("FindByEmail request received", "email", req.Email)

	// Find user by email
	user, err := c.userUseCase.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, c.HandleUseCaseError(err)
	}

	// Convert entity to response
	response, err := c.mapper.EntityToResponse(user)
	if err != nil {
		c.Logger.Error("Failed to convert entity to response", "error", err)
		return nil, status.Error(codes.Internal, "Failed to generate response")
	}

	return response, nil
}

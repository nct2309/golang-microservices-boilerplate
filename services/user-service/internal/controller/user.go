package internal

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	coreTypes "golang-microservices-boilerplate/pkg/core/types"
	"golang-microservices-boilerplate/pkg/core/usecase"
	core_pb "golang-microservices-boilerplate/proto/core"
	pb "golang-microservices-boilerplate/proto/user-service"
	"golang-microservices-boilerplate/services/user-service/internal/model/entity"
	userschema "golang-microservices-boilerplate/services/user-service/internal/model/schema/user"
	userservice_usecase "golang-microservices-boilerplate/services/user-service/internal/usecase"
)

// UserMapper handles mapping between gRPC proto messages and internal types.
type UserMapper struct{}

// EntityToProto converts an entity.User to a proto.User.
func (m *UserMapper) EntityToProto(user *entity.User) (*pb.User, error) {
	if user == nil {
		return nil, errors.New("cannot map nil entity to proto")
	}
	var deletedAt *timestamppb.Timestamp
	if user.DeletedAt != nil {
		deletedAt = timestamppb.New(*user.DeletedAt)
	}
	var lastLoginAt *timestamppb.Timestamp
	if user.LastLoginAt != nil {
		lastLoginAt = timestamppb.New(*user.LastLoginAt)
	}

	return &pb.User{
		Id:          user.ID.String(),
		Username:    user.Username,
		Email:       user.Email,
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		Role:        string(user.Role),
		IsActive:    user.IsActive,
		CreatedAt:   timestamppb.New(user.CreatedAt),
		UpdatedAt:   timestamppb.New(user.UpdatedAt),
		DeletedAt:   deletedAt,
		LastLoginAt: lastLoginAt,
		Phone:       user.Phone,
		Address:     user.Address,
		Age:         user.Age,
		ProfilePic:  user.ProfilePic,
	}, nil
}

// UserDTOToProto converts a userschema.UserResponseDTO to a proto.User.
func (m *UserMapper) UserDTOToProto(dto *userschema.UserResponseDTO) (*pb.User, error) {
	if dto == nil {
		return nil, errors.New("cannot map nil DTO to proto")
	}
	var deletedAt *timestamppb.Timestamp
	if dto.DeletedAt != nil {
		deletedAt = timestamppb.New(*dto.DeletedAt)
	}
	var lastLoginAt *timestamppb.Timestamp
	if dto.LastLoginAt != nil {
		lastLoginAt = timestamppb.New(*dto.LastLoginAt)
	}

	return &pb.User{
		Id:          dto.ID.String(),
		Username:    dto.Username,
		Email:       dto.Email,
		FirstName:   dto.FirstName,
		LastName:    dto.LastName,
		Role:        string(dto.Role),
		IsActive:    dto.IsActive,
		CreatedAt:   timestamppb.New(dto.CreatedAt),
		UpdatedAt:   timestamppb.New(dto.UpdatedAt),
		DeletedAt:   deletedAt,
		LastLoginAt: lastLoginAt,
		Phone:       dto.Phone,
		Address:     dto.Address,
		Age:         int32(dto.Age),
		ProfilePic:  dto.ProfilePic,
	}, nil
}

// ProtoCreateToDTO converts a proto.CreateUserRequest to a userschema.UserCreateDTO.
func (m *UserMapper) ProtoCreateToDTO(req *pb.CreateUserRequest) (userschema.UserCreateDTO, error) {
	if req == nil {
		return userschema.UserCreateDTO{}, errors.New("cannot map nil create request to DTO")
	}
	dto := userschema.UserCreateDTO{
		Username:   req.Username,
		Email:      req.Email,
		Password:   req.Password,
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		Role:       entity.Role(req.Role),
		Phone:      req.Phone,
		Address:    req.Address,
		ProfilePic: req.ProfilePic,
	}
	if req.Age != nil {
		age := int(*req.Age)
		dto.Age = &age
	}
	return dto, nil
}

// ProtoUpdateToDTO converts a proto.UpdateUserRequest to a userschema.UserUpdateDTO.
func (m *UserMapper) ProtoUpdateToDTO(req *pb.UpdateUserRequest) (userschema.UserUpdateDTO, error) {
	if req == nil {
		return userschema.UserUpdateDTO{}, errors.New("cannot map nil update request to DTO")
	}
	dto := userschema.UserUpdateDTO{}

	if req.Username != nil {
		dto.Username = req.Username.Value
	}
	if req.Email != nil {
		dto.Email = req.Email.Value
	}
	if req.FirstName != nil {
		dto.FirstName = req.FirstName.Value
	}
	if req.LastName != nil {
		dto.LastName = req.LastName.Value
	}
	if req.Role != nil {
		role := entity.Role(req.Role.Value)
		if !role.IsValid() {
		} else {
			dto.Role = role
		}
	}
	if req.IsActive != nil {
		dto.IsActive = req.IsActive.Value
	}
	if req.Phone != nil {
		dto.Phone = req.Phone.Value
	}
	if req.Address != nil {
		dto.Address = req.Address.Value
	}
	if req.Age != nil {
		dto.Age = int(req.Age.Value)
	}
	if req.ProfilePic != nil {
		dto.ProfilePic = req.ProfilePic.Value
	}
	return dto, nil
}

// ProtoLoginToSchema converts proto.LoginRequest to userschema.LoginCredentials.
func (m *UserMapper) ProtoLoginToSchema(req *pb.LoginRequest) (userschema.LoginCredentials, error) {
	if req == nil {
		return userschema.LoginCredentials{}, errors.New("cannot map nil login request")
	}
	return userschema.LoginCredentials{
		Email:    req.Email,
		Password: req.Password,
	}, nil
}

// SchemaLoginResultToProto converts userschema.LoginResult to proto.LoginResponse.
func (m *UserMapper) SchemaLoginResultToProto(result *userschema.LoginResult) (*pb.LoginResponse, error) {
	if result == nil {
		return nil, errors.New("cannot map nil login result")
	}
	userProto, err := m.UserDTOToProto(&result.User)
	if err != nil {
		return nil, fmt.Errorf("failed to map user DTO to proto: %w", err)
	}
	return &pb.LoginResponse{
		User:         userProto,
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    result.ExpiresAt,
	}, nil
}

// SchemaRefreshResultToProto converts userschema.RefreshResult to proto.RefreshResponse.
func (m *UserMapper) SchemaRefreshResultToProto(result *userschema.RefreshResult) (*pb.RefreshResponse, error) {
	if result == nil {
		return nil, errors.New("cannot map nil refresh result")
	}
	return &pb.RefreshResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    result.ExpiresAt,
	}, nil
}

// mapProtoValueToGo converts structpb.Value to its corresponding Go type.
func mapProtoValueToGo(v *structpb.Value) interface{} {
	if v == nil {
		return nil
	}
	switch k := v.Kind.(type) {
	case *structpb.Value_NullValue:
		return nil
	case *structpb.Value_NumberValue:
		return k.NumberValue
	case *structpb.Value_StringValue:
		return k.StringValue
	case *structpb.Value_BoolValue:
		return k.BoolValue
	case *structpb.Value_StructValue:
		goMap := make(map[string]interface{}, len(k.StructValue.Fields))
		for fieldName, fieldValue := range k.StructValue.Fields {
			goMap[fieldName] = mapProtoValueToGo(fieldValue)
		}
		return goMap
	case *structpb.Value_ListValue:
		goSlice := make([]interface{}, len(k.ListValue.Values))
		for i, itemValue := range k.ListValue.Values {
			goSlice[i] = mapProtoValueToGo(itemValue)
		}
		return goSlice
	default:
		return nil
	}
}

// ProtoListRequestToFilterOptions converts proto.ListUsersRequest to coreTypes.FilterOptions.
func (m *UserMapper) ProtoListRequestToFilterOptions(req *pb.ListUsersRequest) coreTypes.FilterOptions {
	opts := coreTypes.DefaultFilterOptions()
	if req == nil || req.Options == nil {
		return opts
	}

	if req.Options.Limit != nil {
		opts.Limit = int(*req.Options.Limit)
	}
	if req.Options.Offset != nil {
		opts.Offset = int(*req.Options.Offset)
	}
	if req.Options.SortBy != nil {
		opts.SortBy = *req.Options.SortBy
	}
	if req.Options.SortDesc != nil {
		opts.SortDesc = *req.Options.SortDesc
	}
	if req.Options.IncludeDeleted != nil {
		opts.IncludeDeleted = *req.Options.IncludeDeleted
	}

	if len(req.Options.Filters) > 0 {
		opts.Filters = make(map[string]interface{}, len(req.Options.Filters))
		for k, v := range req.Options.Filters {
			opts.Filters[k] = mapProtoValueToGo(v)
		}
	}
	return opts
}

// PaginationResultToProtoList converts coreTypes.PaginationResult[entity.User] to proto.ListUsersResponse.
func (m *UserMapper) PaginationResultToProtoList(result *coreTypes.PaginationResult[entity.User]) (*pb.ListUsersResponse, error) {
	if result == nil {
		return &pb.ListUsersResponse{
			Users:          []*pb.User{},
			PaginationInfo: &core_pb.PaginationInfo{TotalItems: 0, Limit: 0, Offset: 0},
		}, nil
	}

	usersProto := make([]*pb.User, 0, len(result.Items))
	for _, userEntity := range result.Items {
		userProto, err := m.EntityToProto(userEntity)
		if err != nil {
			return nil, fmt.Errorf("failed to map user entity %s: %w", userEntity.ID, err)
		}
		usersProto = append(usersProto, userProto)
	}

	paginationInfo := &core_pb.PaginationInfo{
		TotalItems: result.TotalItems,
		Limit:      int32(result.Limit),
		Offset:     int32(result.Offset),
	}

	return &pb.ListUsersResponse{
		Users:          usersProto,
		PaginationInfo: paginationInfo,
	}, nil
}

// ProtoUpdateItemToDTO converts a proto.UpdateUserItem to a userschema.UserUpdateDTO.
// Similar to ProtoUpdateToDTO but takes UpdateUserItem as input (doesn't have its own ID field).
func (m *UserMapper) ProtoUpdateItemToDTO(item *pb.UpdateUserItem) (userschema.UserUpdateDTO, error) {
	if item == nil {
		return userschema.UserUpdateDTO{}, errors.New("cannot map nil update item to DTO")
	}
	dto := userschema.UserUpdateDTO{}

	if item.Username != nil {
		dto.Username = item.Username.Value
	}
	if item.Email != nil {
		dto.Email = item.Email.Value
	}
	if item.FirstName != nil {
		dto.FirstName = item.FirstName.Value
	}
	if item.LastName != nil {
		dto.LastName = item.LastName.Value
	}
	if item.Role != nil {
		role := entity.Role(item.Role.Value)
		if role.IsValid() {
			dto.Role = role
		} // else: Ignore invalid role update
	}
	if item.IsActive != nil {
		dto.IsActive = item.IsActive.Value
	}
	if item.Phone != nil {
		dto.Phone = item.Phone.Value
	}
	if item.Address != nil {
		dto.Address = item.Address.Value
	}
	if item.Age != nil {
		dto.Age = int(item.Age.Value)
	}
	if item.ProfilePic != nil {
		dto.ProfilePic = item.ProfilePic.Value
	}
	return dto, nil
}

// --- gRPC Server Implementation ---

// userServer implements the proto.UserServiceServer interface.
type userServer struct {
	pb.UnimplementedUserServiceServer
	uc     userservice_usecase.UserUsecase
	mapper UserMapper
}

// NewUserServer creates a new gRPC server instance.
func NewUserServer(uc userservice_usecase.UserUsecase) pb.UserServiceServer {
	return &userServer{uc: uc, mapper: UserMapper{}}
}

// Create implements proto.UserServiceServer.
func (s *userServer) Create(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	dto, err := s.mapper.ProtoCreateToDTO(req)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to map request: %v", err)
	}

	createdUser, err := s.uc.Create(ctx, dto)
	if err != nil {
		return nil, mapUseCaseErrorToGrpcStatus(err)
	}

	userProto, err := s.mapper.EntityToProto(createdUser)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to map result: %v", err)
	}

	return &pb.CreateUserResponse{User: userProto}, nil
}

// GetByID implements proto.UserServiceServer.
func (s *userServer) GetByID(ctx context.Context, req *pb.GetUserByIDRequest) (*pb.GetUserByIDResponse, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user ID format: %v", err)
	}

	user, err := s.uc.GetByID(ctx, id)
	if err != nil {
		return nil, mapUseCaseErrorToGrpcStatus(err)
	}

	userProto, err := s.mapper.EntityToProto(user)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to map result: %v", err)
	}

	return &pb.GetUserByIDResponse{User: userProto}, nil
}

// List implements proto.UserServiceServer.
func (s *userServer) List(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	opts := s.mapper.ProtoListRequestToFilterOptions(req)

	result, err := s.uc.List(ctx, opts)
	if err != nil {
		return nil, mapUseCaseErrorToGrpcStatus(err)
	}

	response, err := s.mapper.PaginationResultToProtoList(result)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to map result list: %v", err)
	}

	return response, nil
}

// Update implements proto.UserServiceServer.
func (s *userServer) Update(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UpdateUserResponse, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user ID format: %v", err)
	}

	dto, err := s.mapper.ProtoUpdateToDTO(req)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to map update request: %v", err)
	}

	updatedUser, err := s.uc.Update(ctx, id, dto)
	if err != nil {
		return nil, mapUseCaseErrorToGrpcStatus(err)
	}

	userProto, err := s.mapper.EntityToProto(updatedUser)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to map result: %v", err)
	}

	return &pb.UpdateUserResponse{User: userProto}, nil
}

// Delete implements proto.UserServiceServer (handles soft and hard delete).
func (s *userServer) Delete(ctx context.Context, req *pb.DeleteUserRequest) (*emptypb.Empty, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user ID format: %v", err)
	}

	hardDelete := req.GetHardDelete() // Get the flag from the request

	// Call the consolidated use case method
	if err := s.uc.Delete(ctx, id, hardDelete); err != nil {
		return nil, mapUseCaseErrorToGrpcStatus(err)
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
		return nil, mapUseCaseErrorToGrpcStatus(err)
	}

	// Need to map PaginationResult[entity.User] to FindUsersWithFilterResponse
	usersProto := make([]*pb.User, 0, len(result.Items))
	for _, userEntity := range result.Items {
		userProto, mapErr := s.mapper.EntityToProto(userEntity)
		if mapErr != nil {
			return nil, status.Errorf(codes.Internal, "failed to map user entity %s: %v", userEntity.ID, mapErr)
		}
		usersProto = append(usersProto, userProto)
	}

	paginationInfo := &core_pb.PaginationInfo{ // Use corepb alias
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

	dtos := make([]userschema.UserCreateDTO, 0, len(req.Users))
	for i, createReq := range req.Users {
		dto, err := s.mapper.ProtoCreateToDTO(createReq)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "failed to map user %d in bulk request: %v", i, err)
		}
		dtos = append(dtos, dto)
	}

	createdUsers, err := s.uc.CreateMany(ctx, dtos)
	if err != nil {
		return nil, mapUseCaseErrorToGrpcStatus(err)
	}

	usersProto := make([]*pb.User, 0, len(createdUsers))
	for _, userEntity := range createdUsers {
		userProto, mapErr := s.mapper.EntityToProto(userEntity)
		if mapErr != nil {
			return nil, status.Errorf(codes.Internal, "failed to map created user %s: %v", userEntity.ID, mapErr)
		}
		usersProto = append(usersProto, userProto)
	}

	return &pb.CreateUsersResponse{Users: usersProto}, nil
}

// UpdateMany implements proto.UserServiceServer.
func (s *userServer) UpdateMany(ctx context.Context, req *pb.UpdateUsersRequest) (*emptypb.Empty, error) {
	if req == nil || len(req.Items) == 0 {
		return &emptypb.Empty{}, nil // Nothing to update
	}

	// Map proto request items to the map expected by the use case
	updatesMap := make(map[uuid.UUID]userschema.UserUpdateDTO)
	for i, item := range req.Items {
		id, err := uuid.Parse(item.GetId())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid user ID format for item %d: %v", i, err)
		}

		dto, err := s.mapper.ProtoUpdateItemToDTO(item) // Use the new helper
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "failed to map update item %d (ID: %s): %v", i, id, err)
		}
		updatesMap[id] = dto
	}

	// Call the use case
	if err := s.uc.UpdateMany(ctx, updatesMap); err != nil {
		return nil, mapUseCaseErrorToGrpcStatus(err)
	}

	// Return empty response on success
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
			return nil, status.Errorf(codes.InvalidArgument, "invalid user ID format at index %d: %v", i, err)
		}
		uuidSlice = append(uuidSlice, id)
	}

	// Call the consolidated use case method
	if err := s.uc.DeleteMany(ctx, uuidSlice, hardDelete); err != nil {
		return nil, mapUseCaseErrorToGrpcStatus(err)
	}

	return &emptypb.Empty{}, nil
}

// Login implements proto.UserServiceServer.
func (s *userServer) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	creds, err := s.mapper.ProtoLoginToSchema(req)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to map login request: %v", err)
	}

	loginResult, err := s.uc.Login(ctx, creds)
	if err != nil {
		return nil, mapUseCaseErrorToGrpcStatus(err)
	}

	response, err := s.mapper.SchemaLoginResultToProto(loginResult)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to map login result: %v", err)
	}

	return response, nil
}

// Refresh implements proto.UserServiceServer.
func (s *userServer) Refresh(ctx context.Context, req *pb.RefreshRequest) (*pb.RefreshResponse, error) {
	refreshToken := req.GetRefreshToken()
	if refreshToken == "" {
		return nil, status.Errorf(codes.InvalidArgument, "refresh token cannot be empty")
	}

	refreshResult, err := s.uc.Refresh(ctx, refreshToken)
	if err != nil {
		return nil, mapUseCaseErrorToGrpcStatus(err)
	}

	response, err := s.mapper.SchemaRefreshResultToProto(refreshResult)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to map refresh result: %v", err)
	}

	return response, nil
}

// mapUseCaseErrorToGrpcStatus converts use case errors to gRPC status errors.
func mapUseCaseErrorToGrpcStatus(err error) error {
	var ucErr *usecase.UseCaseError
	if errors.As(err, &ucErr) {
		switch ucErr.Type {
		case usecase.ErrNotFound:
			return status.Error(codes.NotFound, ucErr.Message)
		case usecase.ErrInvalidInput:
			return status.Error(codes.InvalidArgument, ucErr.Message)
		case usecase.ErrUnauthorized:
			return status.Error(codes.Unauthenticated, ucErr.Message)
		case usecase.ErrForbidden:
			return status.Error(codes.PermissionDenied, ucErr.Message)
		case usecase.ErrConflict:
			return status.Error(codes.AlreadyExists, ucErr.Message)
		case usecase.ErrInternal:
			return status.Error(codes.Internal, ucErr.Message)
		default:
			return status.Error(codes.Unknown, fmt.Sprintf("unknown use case error: %v", err))
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, err.Error())
	}
	return status.Error(codes.Internal, fmt.Sprintf("an unexpected internal error occurred: %v", err))
}

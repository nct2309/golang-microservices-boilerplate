package controller

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	coreTypes "golang-microservices-boilerplate/pkg/core/types"
	corePb "golang-microservices-boilerplate/proto/core"
	pb "golang-microservices-boilerplate/proto/user-service"
	"golang-microservices-boilerplate/services/user-service/internal/entity"
	userschema "golang-microservices-boilerplate/services/user-service/internal/schema"
	// Keep usecase import only if needed, maybe not needed now
	// userservice_usecase "golang-microservices-boilerplate/services/user-service/internal/usecase"
)

// Mapper defines the interface for mapping between gRPC proto messages and internal types.
type Mapper interface {
	EntityToProto(user *entity.User) (*pb.User, error)
	ProtoCreateToEntity(req *pb.CreateUserRequest) (*entity.User, error)
	ApplyProtoUpdateToEntity(req *pb.UpdateUserRequest, existingUser *entity.User) error
	ProtoLoginToSchema(req *pb.LoginRequest) (userschema.LoginCredentials, error)
	SchemaLoginResultToProto(result *userschema.LoginResult) (*pb.LoginResponse, error)
	SchemaRefreshResultToProto(result *userschema.RefreshResult) (*pb.RefreshResponse, error)
	ProtoListRequestToFilterOptions(req *pb.ListUsersRequest) coreTypes.FilterOptions
	PaginationResultToProtoList(result *coreTypes.PaginationResult[entity.User]) (*pb.ListUsersResponse, error)
}

// Ensure UserMapper implements Mapper interface.
var _ Mapper = (*UserMapper)(nil)

// UserMapper handles mapping between gRPC proto messages and internal types.
type UserMapper struct{}

// NewUserMapper creates a new instance of UserMapper.
func NewUserMapper() *UserMapper {
	return &UserMapper{}
}

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

// ProtoCreateToEntity converts a proto.CreateUserRequest directly to an entity.User pointer.
func (m *UserMapper) ProtoCreateToEntity(req *pb.CreateUserRequest) (*entity.User, error) {
	if req == nil {
		return nil, errors.New("cannot map nil create request to entity")
	}
	user := &entity.User{
		Username:  req.Username,
		Email:     req.Email,
		Password:  req.Password, // Will be hashed by BeforeCreate hook
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Role:      entity.Role(req.Role), // Default applied in BeforeCreate if invalid/empty
		// Safely dereference optional string fields
		Phone:      derefString(req.Phone),
		Address:    derefString(req.Address),
		Age:        req.GetAge(), // Use GetAge() for nil safety
		ProfilePic: derefString(req.ProfilePic),
		IsActive:   false, // Explicitly set default, though BeforeCreate/DB default handles it
	}
	// Role validation can happen here or rely on BeforeCreate hook
	if !user.Role.IsValid() {
		// Decide how to handle invalid roles from proto (e.g., error out or default)
		// Currently, BeforeCreate hook will default it to Officer if invalid.
		// You could return an error here if strict proto validation is needed:
		// return nil, fmt.Errorf("invalid role provided: %s", req.Role)
		return nil, fmt.Errorf("invalid role provided: %s", req.Role)
	}
	// Basic email validation (more in entity.Validate)
	if user.Email == "" || user.FirstName == "" || user.LastName == "" {
		return nil, errors.New("email, first name, and last name are required")
	}

	return user, nil
}

// Helper function to safely dereference string pointers
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ApplyProtoUpdateToEntity applies fields from proto.UpdateUserRequest to an existing entity.User.
// It modifies the existingUser in place.
func (m *UserMapper) ApplyProtoUpdateToEntity(req *pb.UpdateUserRequest, existingUser *entity.User) error {
	if req == nil || existingUser == nil {
		return errors.New("request and existing entity must not be nil")
	}

	// Apply only the fields present in the request
	if req.Username != nil {
		existingUser.Username = req.Username.Value
	}
	if req.Email != nil {
		existingUser.Email = req.Email.Value
	}
	if req.Password != nil {
		// Setting the plain password here; BeforeUpdate hook should handle hashing
		existingUser.Password = req.Password.Value
	}
	if req.FirstName != nil {
		existingUser.FirstName = req.FirstName.Value
	}
	if req.LastName != nil {
		existingUser.LastName = req.LastName.Value
	}
	if req.Role != nil {
		role := entity.Role(req.Role.Value)
		if role.IsValid() {
			existingUser.Role = role
		} else {
			// Optionally return error for invalid role update, or let Validate handle it
			// return fmt.Errorf("invalid role provided for update: %s", req.Role.Value)
		}
	}
	if req.IsActive != nil {
		existingUser.IsActive = req.IsActive.Value
	}
	if req.Phone != nil {
		existingUser.Phone = req.Phone.Value
	}
	if req.Address != nil {
		existingUser.Address = req.Address.Value
	}
	if req.Age != nil {
		existingUser.Age = req.Age.Value
	}
	if req.ProfilePic != nil {
		existingUser.ProfilePic = req.ProfilePic.Value
	}

	return nil // Return nil on success
}

// ProtoLoginToSchema converts proto.LoginRequest to schema.LoginCredentials.
// Update return type and implementation back to schema type
func (m *UserMapper) ProtoLoginToSchema(req *pb.LoginRequest) (userschema.LoginCredentials, error) {
	if req == nil {
		return userschema.LoginCredentials{}, errors.New("cannot map nil login request")
	}
	return userschema.LoginCredentials{
		Email:    req.Email,
		Password: req.Password,
	}, nil
}

// Re-add SchemaLoginResultToProto
// SchemaLoginResultToProto converts userschema.LoginResult to proto.LoginResponse.
func (m *UserMapper) SchemaLoginResultToProto(result *userschema.LoginResult) (*pb.LoginResponse, error) {
	if result == nil {
		return nil, errors.New("cannot map nil login result")
	}
	// LoginResult contains entity.User, map it directly
	userProto, err := m.EntityToProto(&result.User)
	if err != nil {
		return nil, fmt.Errorf("failed to map user entity to proto: %w", err)
	}
	return &pb.LoginResponse{
		User:         userProto,
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    result.ExpiresAt,
	}, nil
}

// Re-add SchemaRefreshResultToProto
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
			PaginationInfo: &corePb.PaginationInfo{TotalItems: 0, Limit: 0, Offset: 0},
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

	paginationInfo := &corePb.PaginationInfo{
		TotalItems: result.TotalItems,
		Limit:      int32(result.Limit),
		Offset:     int32(result.Offset),
	}

	return &pb.ListUsersResponse{
		Users:          usersProto,
		PaginationInfo: paginationInfo,
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

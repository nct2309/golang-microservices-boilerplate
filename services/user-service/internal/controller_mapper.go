package internal

import (
	"time"

	"golang-microservices-boilerplate/pkg/core"
	proto "golang-microservices-boilerplate/proto/user-service"
)

// Explicitly verify that UserControllerMapper implements the core.ControllerMapper interface
var _ core.ControllerMapper[*proto.CreateUserRequest, *proto.UserResponse, User, CreateUserDTO, UpdateUserDTO] = &UserControllerMapper{}

// UserControllerMapper handles mapping between gRPC requests/responses and domain objects
type UserControllerMapper struct{}

// RequestToCreateDTO converts a CreateUserRequest to a CreateUserDTO
func (m *UserControllerMapper) RequestToCreateDTO(req interface{}) (CreateUserDTO, error) {
	createReq, ok := req.(*proto.CreateUserRequest)
	if !ok {
		return CreateUserDTO{}, core.NewUseCaseError(core.ErrInvalidInput, "invalid request type")
	}

	return CreateUserDTO{
		Email:     createReq.Email,
		Password:  createReq.Password,
		FirstName: createReq.FirstName,
		LastName:  createReq.LastName,
		Role:      createReq.Role,
	}, nil
}

// RequestToUpdateDTO converts an UpdateUserRequest to an UpdateUserDTO
func (m *UserControllerMapper) RequestToUpdateDTO(req interface{}) (UpdateUserDTO, error) {
	updateReq, ok := req.(*proto.UpdateUserRequest)
	if !ok {
		return UpdateUserDTO{}, core.NewUseCaseError(core.ErrInvalidInput, "invalid request type")
	}

	dto := UpdateUserDTO{}

	if updateReq.FirstName != nil {
		dto.FirstName = *updateReq.FirstName
	}

	if updateReq.LastName != nil {
		dto.LastName = *updateReq.LastName
	}

	if updateReq.Password != nil {
		dto.Password = *updateReq.Password
	}

	return dto, nil
}

// EntityToResponse converts a User entity to a UserResponse
func (m *UserControllerMapper) EntityToResponse(entity *User) (*proto.UserResponse, error) {
	if entity == nil {
		return nil, nil
	}

	response := &proto.UserResponse{
		Id:        entity.ID.String(),
		Email:     entity.Email,
		FirstName: entity.FirstName,
		LastName:  entity.LastName,
		Role:      string(entity.Role),
		CreatedAt: timestampFromTime(entity.CreatedAt),
		UpdatedAt: timestampFromTime(entity.UpdatedAt),
	}

	return response, nil
}

// EntitiesToListResponse converts a pagination result to a ListUsersResponse
func (m *UserControllerMapper) EntitiesToListResponse(result *core.PaginationResult[User]) (interface{}, error) {
	if result == nil {
		return &proto.ListUsersResponse{}, nil
	}

	users := make([]*proto.UserResponse, 0, len(result.Data))
	for i := range result.Data {
		user, err := m.EntityToResponse(&result.Data[i])
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return &proto.ListUsersResponse{
		Users:  users,
		Total:  int64(result.TotalCount),
		Limit:  int32(result.PageSize),
		Offset: int32(result.Page * result.PageSize),
	}, nil
}

// FilterOptionsFromRequest extracts filter options from a request
func (m *UserControllerMapper) FilterOptionsFromRequest(req interface{}) core.FilterOptions {
	listReq, ok := req.(*proto.ListUsersRequest)
	if !ok {
		return core.DefaultFilterOptions()
	}

	return core.FilterOptions{
		Limit:  int(listReq.Limit),
		Offset: int(listReq.Offset),
		SortBy: "created_at",
	}
}

// LoginRequestToParams extracts login parameters from a LoginRequest
func (m *UserControllerMapper) LoginRequestToParams(req *proto.LoginRequest) (string, string, error) {
	return req.Email, req.Password, nil
}

// UserAndTokenToLoginResponse converts a user and JWT token to a LoginResponse
func (m *UserControllerMapper) UserAndTokenToLoginResponse(user *User, token string, expiresAt time.Time) (*proto.LoginResponse, error) {
	userResp, err := m.EntityToResponse(user)
	if err != nil {
		return nil, err
	}

	return &proto.LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt.Unix(),
		User:      userResp,
	}, nil
}

// timestampFromTime converts a time.Time to an int64 unix timestamp
func timestampFromTime(t time.Time) int64 {
	return t.Unix()
}

// timeFromTimestamp converts an int64 unix timestamp to a time.Time
// func timeFromTimestamp(ts int64) time.Time {
// 	return time.Unix(ts, 0)
// }

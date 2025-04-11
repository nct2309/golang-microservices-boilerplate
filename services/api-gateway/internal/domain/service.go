package domain

// Service represents a microservice definition discovered by the discovery service
type Service struct {
	Name     string `json:"name"`     // Name of the Kubernetes service (e.g., user-service)
	Endpoint string `json:"endpoint"` // gRPC endpoint (e.g., user-service.namespace.svc.cluster.local:50051)
	// Methods field is removed as it's no longer populated by discovery
}

// Method struct is removed as method details are handled by generated code
/*
type Method struct {
	Name           string `json:"name"`
	FullName       string `json:"fullName"`
	InputType      string `json:"inputType"`
	OutputType     string `json:"outputType"`
	IsClientStream bool   `json:"isClientStream"`
	IsServerStream bool   `json:"isServerStream"`
}
*/

// ServiceDiscovery defines the interface for discovering backend services.
type ServiceDiscovery interface {
	// GetAllServices returns all available services (name and endpoint only)
	GetAllServices() ([]Service, error)

	// Close closes all open connections
	Close() error
}

// GrpcClient interface is removed as the gateway directly uses generated clients
/*
type GrpcClient interface {
	// Call executes a gRPC method with the given parameters and returns the result
	Call(ctx context.Context, service, method string, params map[string]interface{}) (map[string]interface{}, error)

	// GetMethodDescriptor fetches detailed information about a gRPC method
	GetMethodDescriptor(service, method string) (*Method, error)
}
*/

// RequestMapper interface is removed as mapping is handled by gRPC-Gateway
/*
type RequestMapper interface {
	// MapRequest converts HTTP request data to gRPC request data
	MapRequest(httpMethod, path string, queryParams map[string]string, body map[string]interface{}) (map[string]interface{}, error)

	// DetermineService determines the target service from the request path
	DetermineService(path string) (string, string, error)
}
*/

// ResponseMapper interface is removed as mapping is handled by gRPC-Gateway
/*
type ResponseMapper interface {
	// MapResponse converts gRPC response data to HTTP response data
	MapResponse(grpcResponse map[string]interface{}, statusCode int) (interface{}, int, error)
}
*/

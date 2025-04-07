package k8s

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"golang-microservices-boilerplate/services/api-gateway/internal/domain"

	google_grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubernetesDiscovery implements the ServiceDiscovery interface for Kubernetes
type KubernetesDiscovery struct {
	client           kubernetes.Interface
	namespace        string
	servicePrefix    string
	refreshInterval  time.Duration
	logger           *log.Logger
	connections      map[string]*google_grpc.ClientConn
	connectionsMutex sync.RWMutex
	services         []domain.Service
	servicesMutex    sync.RWMutex
	done             chan struct{}
}

// DiscoveryOption configures the KubernetesDiscovery
type DiscoveryOption func(*KubernetesDiscovery)

// WithNamespace sets the Kubernetes namespace
func WithNamespace(namespace string) DiscoveryOption {
	return func(kd *KubernetesDiscovery) {
		kd.namespace = namespace
	}
}

// WithServicePrefix sets the prefix for service names to discover
func WithServicePrefix(prefix string) DiscoveryOption {
	return func(kd *KubernetesDiscovery) {
		kd.servicePrefix = prefix
	}
}

// WithRefreshInterval sets the interval for refreshing service discovery
func WithRefreshInterval(interval time.Duration) DiscoveryOption {
	return func(kd *KubernetesDiscovery) {
		kd.refreshInterval = interval
	}
}

// WithLogger sets the logger for discovery
func WithLogger(logger *log.Logger) DiscoveryOption {
	return func(kd *KubernetesDiscovery) {
		kd.logger = logger
	}
}

// NewKubernetesDiscovery creates a new KubernetesDiscovery instance
func NewKubernetesDiscovery(opts ...DiscoveryOption) (*KubernetesDiscovery, error) {
	kd := &KubernetesDiscovery{
		namespace:       "default",        // Default namespace
		refreshInterval: 60 * time.Second, // Default refresh interval
		connections:     make(map[string]*google_grpc.ClientConn),
		services:        []domain.Service{},
		done:            make(chan struct{}),
		logger:          log.Default(),
	}

	// Apply options
	for _, opt := range opts {
		opt(kd)
	}

	// Create Kubernetes client
	config, err := rest.InClusterConfig()
	if err != nil {
		kd.logger.Printf("Failed to get in-cluster config: %v. Falling back to kubeconfig.", err)
		// Fallback to kubeconfig for local development
		kubeconfig := clientcmd.RecommendedHomeFile
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build config from kubeconfig %s: %w", kubeconfig, err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}
	kd.client = clientset

	kd.logger.Println("KubernetesDiscovery initialized")

	// Perform initial discovery
	if err := kd.RefreshConnections(); err != nil {
		kd.logger.Printf("Initial service discovery failed: %v", err)
		// Don't fail initialization, allow retry
	}

	// Start background refresh
	go kd.startBackgroundRefresh()

	return kd, nil
}

// startBackgroundRefresh starts a goroutine to periodically refresh services
func (kd *KubernetesDiscovery) startBackgroundRefresh() {
	ticker := time.NewTicker(kd.refreshInterval)
	defer ticker.Stop()

	kd.logger.Printf("Starting background service discovery refresh every %s", kd.refreshInterval)

	for {
		select {
		case <-ticker.C:
			if err := kd.RefreshConnections(); err != nil {
				kd.logger.Printf("Error during background service refresh: %v", err)
			}
		case <-kd.done:
			kd.logger.Println("Stopping background service discovery refresh")
			return
		}
	}
}

// GetAllServices returns all discovered services
func (kd *KubernetesDiscovery) GetAllServices() ([]domain.Service, error) {
	kd.servicesMutex.RLock()
	defer kd.servicesMutex.RUnlock()
	// Return a copy to prevent modification
	copiedServices := make([]domain.Service, len(kd.services))
	copy(copiedServices, kd.services)
	return copiedServices, nil
}

// GetConnection returns a gRPC connection for a service
func (kd *KubernetesDiscovery) GetConnection(serviceName string) (*google_grpc.ClientConn, error) {
	kd.connectionsMutex.RLock()
	conn, ok := kd.connections[serviceName]
	kd.connectionsMutex.RUnlock()

	if !ok {
		// Attempt to refresh if connection not found, might have just appeared
		kd.logger.Printf("Connection for service %s not found, attempting refresh...", serviceName)
		if err := kd.RefreshConnections(); err != nil {
			return nil, fmt.Errorf("failed to refresh connections while getting connection for %s: %w", serviceName, err)
		}
		kd.connectionsMutex.RLock()
		conn, ok = kd.connections[serviceName]
		kd.connectionsMutex.RUnlock()
		if !ok {
			return nil, fmt.Errorf("service %s not found after refresh", serviceName)
		}
	}
	return conn, nil
}

// Close closes all connections and stops background refresh
func (kd *KubernetesDiscovery) Close() error {
	kd.logger.Println("Closing KubernetesDiscovery...")
	close(kd.done) // Signal background refresh to stop

	kd.connectionsMutex.Lock()
	defer kd.connectionsMutex.Unlock()

	var closeErrors []string
	for name, conn := range kd.connections {
		if err := conn.Close(); err != nil {
			errMsg := fmt.Sprintf("failed to close connection to %s: %v", name, err)
			kd.logger.Println(errMsg)
			closeErrors = append(closeErrors, errMsg)
		}
		delete(kd.connections, name)
	}

	if len(closeErrors) > 0 {
		return fmt.Errorf("errors closing connections: %s", strings.Join(closeErrors, "; "))
	}
	kd.logger.Println("KubernetesDiscovery closed successfully")
	return nil
}

// RefreshConnections refreshes all service connections
func (kd *KubernetesDiscovery) RefreshConnections() error {
	services, err := kd.discoverServices()
	if err != nil {
		return fmt.Errorf("failed to discover services: %w", err)
	}

	// Store the discovered services (only Name and Endpoint now)
	kd.servicesMutex.Lock()
	kd.services = services
	kd.servicesMutex.Unlock()

	// Get current connections
	kd.connectionsMutex.Lock()
	defer kd.connectionsMutex.Unlock()

	// Track which connections should be kept
	activeConnections := make(map[string]bool)

	// Create or update connections for each service
	for _, service := range services {
		activeConnections[service.Name] = true

		// Check if we already have a connection
		if _, exists := kd.connections[service.Name]; !exists {
			// Create a new connection
			conn, err := google_grpc.NewClient(service.Endpoint, google_grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				kd.logger.Printf("Failed to create gRPC connection to %s (%s): %v", service.Name, service.Endpoint, err)
				continue // Try next service
			}

			kd.connections[service.Name] = conn
			kd.logger.Printf("Established connection to service: %s at %s", service.Name, service.Endpoint)
		}

		// REMOVED: No longer discover methods using reflection here
		// methods, err := kd.reflectionClient.GetMethodsForService(conn, service.Name)
		// ... error handling ...
		// kd.services[i].Methods = methods
		// kd.logger.Printf("Discovered %d methods for service: %s", len(methods), service.Name)
	}

	// Close connections that are no longer active
	for name, conn := range kd.connections {
		if !activeConnections[name] {
			if err := conn.Close(); err != nil {
				kd.logger.Printf("Failed to close inactive connection to %s: %v", name, err)
			}
			delete(kd.connections, name)
			kd.logger.Printf("Closed inactive connection to service: %s", name)
		}
	}

	return nil
}

// discoverServices discovers all available services from Kubernetes
func (kd *KubernetesDiscovery) discoverServices() ([]domain.Service, error) {
	// List services in the namespace
	serviceList, err := kd.client.CoreV1().Services(kd.namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	var services []domain.Service

	// Process each service
	for _, svc := range serviceList.Items {
		// Skip if doesn't have our prefix
		if !kd.hasPrefix(svc.Name, kd.servicePrefix) {
			continue
		}

		// Skip if no ports
		if len(svc.Spec.Ports) == 0 {
			continue
		}

		// Find the gRPC port
		var port int32
		for _, p := range svc.Spec.Ports {
			if p.Name == "grpc" || p.Port == 50051 {
				port = p.Port
				break
			}
		}

		// If no gRPC port found, use the first one
		if port == 0 && len(svc.Spec.Ports) > 0 {
			port = svc.Spec.Ports[0].Port
		}

		// Skip if still no port
		if port == 0 {
			continue
		}

		// Create endpoint
		endpoint := fmt.Sprintf("%s.%s.svc.cluster.local:%d", svc.Name, kd.namespace, port)

		// Service name is the Kubernetes service name
		serviceName := svc.Name

		// Create service entry
		service := domain.Service{
			Name:     serviceName,
			Endpoint: endpoint,
			// REMOVED: Methods field is no longer populated here
			// Methods:  []domain.Method{},
		}

		services = append(services, service)
		kd.logger.Printf("Discovered service: %s at %s", serviceName, endpoint)
	}

	return services, nil
}

// hasPrefix checks if a service name has the specified prefix
func (kd *KubernetesDiscovery) hasPrefix(name, prefix string) bool {
	if prefix == "" {
		return true
	}
	return strings.HasPrefix(name, prefix)
}

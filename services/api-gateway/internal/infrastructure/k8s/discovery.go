package k8s

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	// "time" // No longer needed for refresh interval

	"golang-microservices-boilerplate/services/api-gateway/internal/domain"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubernetesDiscovery implements the ServiceDiscovery interface for Kubernetes
// It discovers service names and endpoints once at initialization.
type KubernetesDiscovery struct {
	client        kubernetes.Interface
	namespace     string
	servicePrefix string
	logger        *log.Logger
	services      []domain.Service
	servicesMutex sync.RWMutex // Mutex for services slice
	// done channel removed
	// refreshInterval removed
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

// WithLogger sets the logger for discovery
func WithLogger(logger *log.Logger) DiscoveryOption {
	return func(kd *KubernetesDiscovery) {
		kd.logger = logger
	}
}

// NewKubernetesDiscovery creates a new KubernetesDiscovery instance
// and performs service discovery once.
func NewKubernetesDiscovery(opts ...DiscoveryOption) (*KubernetesDiscovery, error) {
	kd := &KubernetesDiscovery{
		namespace: "default",          // Default namespace
		services:  []domain.Service{}, // Initialize services slice
		logger:    log.Default(),
		// refreshInterval removed
		// done channel removed
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

	kd.logger.Println("KubernetesDiscovery initializing...")

	// Perform initial discovery immediately
	if err := kd.discoverAndStoreServices(); err != nil {
		// Log the error but potentially allow startup if services might appear later?
		// Or return the error to prevent startup if services must exist initially.
		return nil, fmt.Errorf("initial service discovery failed: %w", err)
	}

	// No background refresh started
	kd.logger.Println("KubernetesDiscovery initialized successfully.")
	return kd, nil
}

// GetAllServices returns all discovered services found during initialization
func (kd *KubernetesDiscovery) GetAllServices() ([]domain.Service, error) {
	kd.servicesMutex.RLock()
	defer kd.servicesMutex.RUnlock()
	// Return a copy to prevent modification
	copiedServices := make([]domain.Service, len(kd.services))
	copy(copiedServices, kd.services)
	// Error is always nil now as it's just returning the stored slice
	return copiedServices, nil
}

// Close is now a no-op as there are no background tasks or connections to close.
func (kd *KubernetesDiscovery) Close() error {
	kd.logger.Println("Closing KubernetesDiscovery (no-op).")
	return nil
}

// discoverAndStoreServices discovers services once and stores them.
// Renamed from RefreshConnections.
func (kd *KubernetesDiscovery) discoverAndStoreServices() error {
	kd.logger.Println("Discovering Kubernetes services...")
	services, err := kd.discoverServices()
	if err != nil {
		return fmt.Errorf("failed to discover services: %w", err)
	}

	// Store the discovered services
	kd.servicesMutex.Lock()
	kd.services = services
	kd.servicesMutex.Unlock()

	kd.logger.Printf("Successfully discovered and stored %d services.", len(services))
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
			kd.logger.Printf("Service %s skipped: No ports defined.", svc.Name)
			continue
		}

		// Find the gRPC port
		var port int32
		for _, p := range svc.Spec.Ports {
			if p.Name == "grpc" || p.Port == 50051 { // Common gRPC port names/numbers
				port = p.Port
				break
			}
		}

		// If no specifically named gRPC port found, use the first one
		if port == 0 && len(svc.Spec.Ports) > 0 {
			port = svc.Spec.Ports[0].Port
			kd.logger.Printf("Service %s: No 'grpc' or '50051' port found, using first port: %d", svc.Name, port)
		}

		// Skip if still no port
		if port == 0 {
			kd.logger.Printf("Service %s skipped: No suitable port found (looked for 'grpc', 50051, or first port).", svc.Name)
			continue
		}

		// Create endpoint (adjust if using ClusterIP, NodePort, or LoadBalancer differently)
		// This assumes ClusterIP service type and internal cluster DNS resolution.
		endpoint := fmt.Sprintf("%s.%s.svc.cluster.local:%d", svc.Name, kd.namespace, port)

		// Service name is the Kubernetes service name
		serviceName := svc.Name

		// Create service entry
		service := domain.Service{
			Name:     serviceName,
			Endpoint: endpoint,
		}

		services = append(services, service)
		kd.logger.Printf("Discovered service: %s at %s", serviceName, endpoint)
	}

	// Handle case where no services are found
	if len(services) == 0 {
		kd.logger.Printf("WARN: No services found matching prefix '%s' in namespace '%s'", kd.servicePrefix, kd.namespace)
	}

	return services, nil
}

// hasPrefix checks if a service name has the specified prefix
func (kd *KubernetesDiscovery) hasPrefix(name, prefix string) bool {
	if prefix == "" {
		return true // If no prefix specified, match all
	}
	return strings.HasPrefix(name, prefix)
}

install-deps:
	# Install Buf CLI
	BUF_VERSION="1.14.0" && \
	echo "Installing Buf CLI v$${BUF_VERSION}..." && \
	curl -sSL "https://github.com/bufbuild/buf/releases/download/v$${BUF_VERSION}/buf-$$(uname -s)-$$(uname -m)" -o buf && \
	chmod +x buf && \
	sudo mv buf /usr/local/bin/buf && \
	echo "Buf CLI installed successfully."

	# Install gRPC-Gateway dependencies
	echo "Installing Go dependencies..." && \
	go install github.com/grpc-ecosystem/grpc-gateway/v2/runtime && \
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway && \
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2 && \
	go install google.golang.org/protobuf/cmd/protoc-gen-go && \
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc

	# Create Swagger UI directory
	echo "Setting up Swagger UI..." && \
	mkdir -p services/api-gateway/swagger/swagger-ui

	# Download Swagger UI
	curl -sSL https://github.com/swagger-api/swagger-ui/archive/v4.15.5.tar.gz | tar -xz --strip-components=2 -C services/api-gateway/swagger/swagger-ui swagger-ui-4.15.5/dist

	# Update Swagger UI configuration
	sed -i 's|https://petstore.swagger.io/v2/swagger.json|/swagger/openapi.json|g' services/api-gateway/swagger/swagger-ui/swagger-initializer.js
	
create-cluster:
	kind create cluster --config k8s/cluster-setup/kind-config.yaml

delete-cluster:
	kind delete cluster --name ride-sharing-cluster

remove-image:
	docker rmi api-gateway:latest
	docker rmi user-service:latest
	docker rmi water-quality-service:latest

build-image:
	docker build -t api-gateway:latest -f services/api-gateway/Dockerfile .
	docker build -t user-service:latest -f services/user-service/Dockerfile .
	docker build -t water-quality-service:latest -f services/water-quality-service/Dockerfile .

image:	remove-image	build-image

load-image:
	kind load docker-image api-gateway:latest --name ride-sharing-cluster
	kind load docker-image user-service:latest --name ride-sharing-cluster
	kind load docker-image water-quality-service:latest --name ride-sharing-cluster

apply-config:
	kubectl apply -f k8s/common/ # Apply Namespace and RBAC
	kubectl apply -f k8s/api-gateway/ # Apply ConfigMap, Deployment, Service, Ingress
	kubectl apply -f k8s/user-service/ # Apply ConfigMap, Deployment, Service
	kubectl apply -f k8s/water-quality-service/ # Apply ConfigMap, Deployment, Service

.PHONY: describe-api
describe-api:
	kubectl describe pod -n ride-sharing -l app=api-gateway

.PHONY: describe-user
describe-user:
	kubectl describe pod -n ride-sharing -l app=user-service

.PHONY: describe-water-quality
	kubectl describe pod -n ride-sharing -l app=water-quality-service

.PHONY: api-logs
api-logs:
	kubectl logs -n ride-sharing -l app=api-gateway --tail=100

.PHONY: user-logs
user-logs:
	kubectl logs -n ride-sharing -l app=user-service --tail=100

.PHONY: water-quality-logs
water-quality-logs:
	kubectl logs -n ride-sharing -l app=water-quality-service --tail=100

.PHONY: restart-deployments
restart-deployments:
	kubectl rollout restart deployment -n ride-sharing api-gateway
	kubectl rollout restart deployment -n ride-sharing user-service
	kubectl rollout restart deployment -n ride-sharing water-quality-service
forward-api:
	kubectl port-forward -n ride-sharing service/api-gateway 8081:8081

proto-gen:
	buf generate

clear-docker-cache:
	docker builder prune -f

# move-out:
# 	cp -r golang-microservices-boilerplate /mnt/c/Users/ASUS/Downloads
# move-in:
# 	cp -r /mnt/c/Users/ASUS/Downloads/golang-microservices-boilerplate .
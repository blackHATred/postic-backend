.PHONY: migration-file
migration-file:
	@if [ -z "$(MIGRATION_NAME)" ]; then \
		echo "Error: MIGRATION_NAME is not set. Use: make migration-file MIGRATION_NAME=<name>"; \
		exit 1; \
	fi
	@TIMESTAMP=$$(date +%Y%m%d_%H%M%S) && \
	FILENAME="R__$${TIMESTAMP}_$(MIGRATION_NAME).sql" && \
	touch "cockroachdb/migrations/$${FILENAME}" && \
	echo "Migration file created: $${FILENAME}"

.PHONY: run-docker-compose
run-docker-compose:
	docker compose up --build -d

.PHONY: gen-nginx-certs
gen-nginx-certs:
	@echo "Generating self-signed certificates for Nginx..."
	@mkdir -p nginx/certs
	@openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
		-keyout nginx/certs/nginx.key \
		-out nginx/certs/nginx.crt
	@echo "Certificates generated in the 'certs' directory."

.PHONY: push-docker
push-docker:
	@echo "Pushing Docker images to Docker Hub..."
	docker build -t ghcr.io/blackhatred/postic-backend:latest .
	docker push ghcr.io/blackhatred/postic-backend:latest
	@echo "Docker images pushed successfully."

.PHONY: expose-k8s
expose-k8s:
	go run cmd/utils/expose-k8s/main.go \
		--kubeconfig kubeconfig \
		--svc kafka-cluster-kafka-bootstrap:kafka:9092:9092 \
		--svc cockroachdb-public:cockroachdb:26257:26257 \
		--svc minio:minio:9000:9000 \
		--svc user-service:postic:50051:50051 \
		--svc kubernetes-dashboard-kong-proxy:kubernetes-dashboard:8443:443

.PHONY: run-vk-listener
run-vk-listener:
	@echo "Running VK Listener..."
	go run .\cmd\vk-event-listener\main.go

.PHONY: run-telegram-listener
run-telegram-listener:
	@echo "Running Telegram Listener..."
	go run .\cmd\telegram-event-listener\main.go

.PHONY: push-telegram-listener-container
push-telegram-listener-container:
	@echo "Building and pushing Telegram Listener Docker container..."
	docker build -t ghcr.io/blackhatred/postic-telegram-listener:latest -f cmd/telegram-event-listener/Dockerfile .
	docker push ghcr.io/blackhatred/postic-telegram-listener:latest
	@echo "Telegram Listener container pushed successfully."

.PHONY: push-vk-listener-container
push-vk-listener-container:
	@echo "Building and pushing VK Listener Docker container..."
	docker build -t ghcr.io/blackhatred/postic-vk-listener:latest -f cmd/vk-event-listener/Dockerfile .
	docker push ghcr.io/blackhatred/postic-vk-listener:latest
	@echo "VK Listener container pushed successfully."

.PHONY: run-stats-worker
run-stats-worker:
	@echo "Running Stats Worker..."
	go run .\cmd\stats-worker\main.go

.PHONY: push-stats-worker-container
push-stats-worker-container:
	@echo "Building and pushing Stats Worker Docker container..."
	docker build -t ghcr.io/blackhatred/postic-stats-worker:latest -f cmd/stats-worker/Dockerfile .
	docker push ghcr.io/blackhatred/postic-stats-worker:latest
	@echo "Stats Worker container pushed successfully."

.PHONY: run-user-service
run-user-service:
	@echo "Running User Service..."
	go run .\cmd\user-service\main.go

.PHONY: push-user-service-container
push-user-service-container:
	@echo "Building and pushing User Service Docker container..."
	docker build -t ghcr.io/blackhatred/postic-user-service:latest -f cmd/user-service/Dockerfile .
	docker push ghcr.io/blackhatred/postic-user-service:latest
	@echo "User Service container pushed successfully."

.PHONY: run-gateway
run-gateway:
	@echo "Running Gateway..."
	go run .\cmd\gateway\main.go

.PHONY: push-gateway-container
push-gateway-container:
	@echo "Building and pushing Gateway Docker container..."
	docker build -t ghcr.io/blackhatred/postic-gateway:latest -f cmd/gateway/Dockerfile .
	docker push ghcr.io/blackhatred/postic-gateway:latest
	@echo "Gateway container pushed successfully."

.PHONY: proto-gen
proto-gen:
	go run cmd/utils/proto-gen/main.go

.PHONY: push-upload-service
push-upload-service:
	@echo "Building and pushing upload-service Docker image to GHCR..."
	docker build -t ghcr.io/blackhatred/postic-upload-service:latest -f cmd/upload-service/Dockerfile .
	docker push ghcr.io/blackhatred/postic-upload-service:latest
	@echo "upload-service image pushed successfully."

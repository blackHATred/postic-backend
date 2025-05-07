.PHONY: migration-file
migration-file:
	@if [ -z "$(MIGRATION_NAME)" ]; then \
		echo "Error: MIGRATION_NAME is not set. Use: make migration-file MIGRATION_NAME=<name>"; \
		exit 1; \
	fi
	@TIMESTAMP=$$(date +%Y%m%d_%H%M%S) && \
	FILENAME="$R__${TIMESTAMP}_$(MIGRATION_NAME).sql" && \
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

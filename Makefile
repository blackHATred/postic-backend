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

.PHONY: install-foliant
install-foliant:
	pip3 install foliant foliantcontrib.init
	pip3 install foliantcontrib.pandoc

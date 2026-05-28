.PHONY: mountabo backend frontend deps

# Start the whole project: backend + frontend together.
# Ctrl-C stops both.
mountabo:
	@echo "starting mountabo (backend + frontend)..."
	@trap 'kill 0' EXIT INT TERM; \
	$(MAKE) backend & \
	$(MAKE) frontend & \
	wait

backend:
	@cd backend && go run ./cmd/server

frontend:
	@cd frontend && npm run dev

deps:
	@cd backend && go mod download
	@cd frontend && npm install

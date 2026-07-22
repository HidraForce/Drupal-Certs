.PHONY: dev api web test
dev:
	docker compose up --build
web:
	cd web && npm start
test:
	cd services/quiz && go test ./...
	cd services/progress && go test ./...
	cd web && npm run build

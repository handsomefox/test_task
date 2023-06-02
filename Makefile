test:
	@go test -v -p 1 ./...

start_db:
	@docker compose -f docker-compose-pg.yaml up

stop_db:
	@docker compose -f docker-compose-pg.yaml down

.PHONY: run test tidy db-shell db-reset

run:
	go run ./cmd/server

test:
	go test ./...

tidy:
	go mod tidy

# Open a psql shell inside the Postgres container.
db-shell:
	docker exec -it triagely-db psql -U triagely -d triagely

# Delete all data and start fresh. The server re-runs migrations on start.
db-reset:
	docker exec -i triagely-db psql -U triagely -d triagely -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"

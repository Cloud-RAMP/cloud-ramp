CC=go
JS_CC=node
BINARY=bin/cloud-ramp
MAIN=cmd/cloud-ramp/main.go
CLIENT=ts-client/client.ts

build:
	@mkdir -p $(dir $(BINARY))
	$(CC) build -o $(BINARY) $(MAIN)

run:
	$(CC) run $(MAIN)

run-client:
	$(JS_CC) $(CLIENT)

build-docker:
	docker build -t cloud-ramp .

run-docker:
	docker run -p 8080:8080 --env-file .env cloud-ramp

clean:
	rm build
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
	docker build -t peterolsen1/cloud-ramp:1.0.0 .

run-docker:
	docker run -p 8080:8080 --env-file .env peterolsen1/cloud-ramp:1.0.0

clean:
	rm build
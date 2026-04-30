CC=go
JS_CC=node
BINARY=bin/cloud-ramp
MAIN=cmd/cloud-ramp/main.go
CLIENT=ts-client/client.ts
TAG?=1.0.0

build:
	@mkdir -p $(dir $(BINARY))
	$(CC) build -o $(BINARY) $(MAIN)

run:
	$(CC) run $(MAIN)

run-client:
	$(JS_CC) $(CLIENT)

build-docker:
	docker build -t peterolsen1/cloud-ramp:$(TAG) .

run-docker:
	docker run -p 8080:8080 --env-file .env peterolsen1/cloud-ramp:$(TAG)

push-docker: build-docker
	docker tag peterolsen1/cloud-ramp:$(TAG) peterolsen1/cloud-ramp:$(TAG)
	docker tag peterolsen1/cloud-ramp:$(TAG) peterolsen1/cloud-ramp:latest

	docker push peterolsen1/cloud-ramp:$(TAG)
	docker push peterolsen1/cloud-ramp:latest

clean:
	rm build

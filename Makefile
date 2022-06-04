BINARY=engine
test: 
	go test -v -cover -covermode=atomic ./...

engine:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ${BINARY} ./main.go

unittest:
	go test -short  ./...

test-coverage:
	go test -short -coverprofile cover.out -covermode=atomic ./...
	cat cover.out >> coverage.txt

clean:
	if [ -f ${BINARY} ] ; then rm ${BINARY} ; fi

docker:
	docker build -t payment-service .

run:
	docker-compose up -d

stop:
	docker-compose down

lint-prepare:
	@echo "Installing golangci-lint" 
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s latest

lint:
	./bin/golangci-lint run \
		--exclude-use-default=false \
		--enable=revive \
		--enable=gocyclo \
		--enable=goconst \
		--enable=unconvert \
		./...

.PHONY: test engine unittest test-coverage clean docker run stop lint-prepare lint
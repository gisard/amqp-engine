PROJECTNAME="amqp-engine"

GOCMD=go
GOPROXY=GOPROXY=https://goproxy.cn GONOSUMDB='*'
GOBUILD=$(GOPROXY) $(GOCMD) build
GOMODTIDY=$(GOPROXY) $(GOCMD) mod tidy
GOLINT=golangci-lint run

deps:
	$(GOMODTIDY)
	$(GOBUILD) ./... ./...

proj:
	$(GOMODTIDY)
	$(GOBUILD) ./... ./...
	$(GOLINT) ./...

mod_tidy:
	$(GOMODTIDY)

## lint: run golint in all packages
lint:
	@$(GOLINT) ./...

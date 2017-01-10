NAME:=migu
ORGANIZATION:=astronoka
PROJECT_PATH:=github.com/$(ORGANIZATION)/$(NAME)

BUILD_DIR:=$(CURDIR)/_build
# output binary directory
ARTIFACT_BIN_DIR:=$(BUILD_DIR)/bin
# fake target file directory
FAKE_TARGET_DIR:=$(BUILD_DIR)/fake
# container fake target file directory
CONTAINER_TARGET_DIR:=$(FAKE_TARGET_DIR)/container
# vendoring fake target file path
VENDORING_TARGET:=$(FAKE_TARGET_DIR)/vendoring

# docker image for build
BUILD_CONTAINER_TARGET:=$(CONTAINER_TARGET_DIR)/build-go-package
BUILD_CONTAINER_IMAGE:=$(ORGANIZATION)/build-go-package:latest

# default docker network (use `bridge` if not specify)
DOCKER_NETWORK?=bridge

# run build container shortcut
WITH_BUILD_CONTAINER:=docker run --rm \
	--volume $(CURDIR):/go/src/$(PROJECT_PATH) \
	--volume $(ARTIFACT_BIN_DIR):/go/bin \
	--workdir /go/src/$(PROJECT_PATH) \
	--network $(DOCKER_NETWORK) \
	$(BUILD_CONTAINER_IMAGE)

# go src files without vendoring
GO_SRCS:=$(shell find $(CURDIR) -type f -name '*.go' -not -path "$(CURDIR)/vendor/*" -not -path "$(CURDIR)/.glide/*")

BRANCH_NAME=$(shell git rev-parse --abbrev-ref HEAD)
COMMIT_HASH=$(shell git rev-parse --short HEAD)
VERSION=$(shell git describe --tags 2&>/dev/null  || echo 'v0.0.0')

GO_LDFLAGS= -X 'main.version=$(VERSION)' \
	-X 'main.commitHash=$(COMMIT_HASH)'

# cmd/* directory -> bin name
CMD_PACKAGES:=$(shell find $(CURDIR)/cmd/* -type d)
CMD_NAMES:=$(notdir $(CMD_PACKAGES))
CMD_BINS:=$(addprefix $(ARTIFACT_BIN_DIR)/, $(CMD_NAMES))

.PHONY: all
all: $(CMD_BINS)

$(ARTIFACT_BIN_DIR)/%: $(BUILD_CONTAINER_TARGET) $(VENDORING_TARGET) $(GO_SRCS)
	@echo ✨ Build $(@F)
	mkdir -p $(@D)
	$(WITH_BUILD_CONTAINER) go build -ldflags "$(GO_LDFLAGS)" -v -o /go/bin/$(@F) $(PROJECT_PATH)/cmd/$(@F)

$(VENDORING_TARGET): $(BUILD_CONTAINER_TARGET) glide.yaml
	@echo ✨ Vendoring
	$(WITH_BUILD_CONTAINER) glide install -v \
		&& mkdir -p $(@D) \
		&& touch $(VENDORING_TARGET)

$(BUILD_CONTAINER_TARGET): Dockerfile.build
	@echo ✨ Build $(BUILD_CONTAINER_IMAGE)
	docker build -t $(BUILD_CONTAINER_IMAGE) -f Dockerfile.build . \
		&& mkdir -p $(@D) \
		&& touch $(BUILD_CONTAINER_TARGET)

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)

.PHONY: distclean
distclean: clean
	docker rmi $(BUILD_CONTAINER_IMAGE)

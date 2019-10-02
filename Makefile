all: build
.PHONY: all


# You can customize go tools depending on the directory layout.
# example:
GO_BUILD_PACKAGES :=./pkg/...
# You can list all the golang related variables by:
#   $ make -n --print-data-base | grep ^GO

# Include the library makefile
OS_LIBRARY_GO_VERSION :=v0.0.0-20190927184318-c355e2019bb3

include $(GOPATH)/pkg/mod/github.com/openshift/library-go@$(OS_LIBRARY_GO_VERSION)/alpha-build-machinery/make/golang.mk
# All the available targets are listed in <this-file>.help
# or you can list it live by using `make help`

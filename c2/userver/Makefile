BINS := $(filter-out %_test.go,$(notdir $(wildcard cmd/*)))

all: build
build: $(BINS)

.PHONY: $(addprefix bin/,$(BINS))
$(addprefix bin/,$(BINS)):
	go build -buildmode=pie -trimpath -o $@ ./cmd/$(@F)

$(BINS): $(addprefix bin/,$(BINS))

PREFIX ?= /usr
DESTDIR ?=
GO ?= go
BINARY := build/ryoport

.PHONY: build test vet fmt-check changelog changelog-check install tarball

build:
	mkdir -p build
	CGO_ENABLED=1 $(GO) build -trimpath -ldflags="-s -w" -o $(BINARY) ./cmd/ryoport

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt-check:
	@test -z "$$(gofmt -l $$(find . -path ./vendor -prune -o -name '*.go' -print))"

changelog:
	cp CHANGELOG.md internal/version/CHANGELOG.md

changelog-check:
	cmp -s CHANGELOG.md internal/version/CHANGELOG.md

install:
ifndef SKIP_BUILD
	$(MAKE) build
endif
	install -Dm755 $(BINARY) "$(DESTDIR)$(PREFIX)/bin/ryoport"
	ln -sfn ryoport "$(DESTDIR)$(PREFIX)/bin/ryovm"
	ln -sfn ryoport "$(DESTDIR)$(PREFIX)/bin/ryossh"
	ln -sfn ryoport "$(DESTDIR)$(PREFIX)/bin/ryovm-mon"
	ln -sfn ryoport "$(DESTDIR)$(PREFIX)/bin/ryovm-fetch"
	install -Dm644 packaging/linux/dev.ryoku.ryoport.desktop "$(DESTDIR)$(PREFIX)/share/applications/dev.ryoku.ryoport.desktop"
	install -Dm644 packaging/linux/dev.ryoku.ryoport.svg "$(DESTDIR)$(PREFIX)/share/icons/hicolor/scalable/apps/dev.ryoku.ryoport.svg"
	install -Dm644 packaging/linux/dev.ryoku.ryoport.metainfo.xml "$(DESTDIR)$(PREFIX)/share/metainfo/dev.ryoku.ryoport.metainfo.xml"
	install -Dm644 LICENSE "$(DESTDIR)$(PREFIX)/share/licenses/ryoport/LICENSE"

tarball:
	scripts/build-tarball.sh dist

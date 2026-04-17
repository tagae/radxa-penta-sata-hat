PACKAGE  := radxa-penta
VERSION  ?= 0.3

# Debian architecture: arm64 (RPi 4/5 64-bit, Rock Pi), armhf (RPi 32-bit), amd64
ARCH     ?= arm64

# ---------- Map Debian arch → Go cross-compilation vars ----------
ifeq ($(ARCH),arm64)
  GOARCH := arm64
else ifeq ($(ARCH),armhf)
  GOARCH := arm
  export GOARM := 7
else ifeq ($(ARCH),amd64)
  GOARCH := amd64
else
  $(error Unsupported ARCH=$(ARCH). Use arm64, armhf, or amd64)
endif

# ---------- Build directories ----------
BUILDDIR := build/$(PACKAGE)
DEB      := $(PACKAGE)_$(VERSION)_$(ARCH).deb
BIN      := $(PACKAGE)

FONTS    := $(wildcard fonts/*.ttf)
ENVFILES := $(wildcard env/*.env)
DEBFILES := debian/control debian/postinst debian/prerm debian/templates \
			debian/conffiles debian/radxa-penta.service

.PHONY: all build deb clean get tidy format vetto

all: deb

# Cross-compile a statically-linked binary for the target.
build: $(BIN)

$(BIN): *.go go.mod go.sum
	GOOS=linux GOARCH=$(GOARCH) CGO_ENABLED=0 \
		go build -trimpath -ldflags='-s -w' -o $@ .

# Assemble the deb tree and package it.
deb: $(DEB)

$(DEB): $(BIN) $(FONTS) $(ENVFILES) $(DEBFILES) radxa-penta.conf
	rm -rf $(BUILDDIR)

	# -- DEBIAN metadata --
	mkdir -p $(BUILDDIR)/DEBIAN
	sed 's/%%ARCH%%/$(ARCH)/g; s/%%VERSION%%/$(VERSION)/g' \
		debian/control > $(BUILDDIR)/DEBIAN/control
	install -m 755 debian/postinst  $(BUILDDIR)/DEBIAN/
	install -m 755 debian/prerm     $(BUILDDIR)/DEBIAN/
	install -m 644 debian/templates $(BUILDDIR)/DEBIAN/
	install -m 644 debian/conffiles $(BUILDDIR)/DEBIAN/

	# -- Binary --
	install -d            $(BUILDDIR)/usr/bin
	install -m 755 $(BIN) $(BUILDDIR)/usr/bin/

	# -- Fonts --
	install -d $(BUILDDIR)/usr/share/radxa-penta/fonts
	install -m 644 $(FONTS) $(BUILDDIR)/usr/share/radxa-penta/fonts/

	# -- Hardware env files --
	install -d $(BUILDDIR)/usr/share/radxa-penta/env
	install -m 644 env/*.env \
		$(BUILDDIR)/usr/share/radxa-penta/env/

	# -- Configuration --
	install -d          $(BUILDDIR)/etc
	install -m 644 radxa-penta.conf $(BUILDDIR)/etc/

	# -- Systemd service --
	install -d          $(BUILDDIR)/lib/systemd/system
	install -m 644 debian/radxa-penta.service \
		$(BUILDDIR)/lib/systemd/system/

	# -- Build the .deb --
	dpkg-deb --build --root-owner-group -Z gzip $(BUILDDIR)
	mv build/$(PACKAGE).deb $@
	@echo "Built $@"

clean:
	rm -rf build $(BIN) *.deb

get:
	go get ./...

format:
	go fmt ./...

tidy:
	go mod tidy

vetto:
	GOOS=linux GOARCH=$(GOARCH) go vet ./...

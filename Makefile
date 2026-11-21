.PHONY: web gtk test test-gtk smoke-gtk package

web:
	go build -o dvd-audio-studio .

# GTK 3.20 API bindings run on GTK 3.24 and avoid gotk3 v0.6.4's
# broken gdk_since_3_22.go callback import. This does not downgrade GTK.
gtk:
	bash -c 'if ! pkg-config --exists gtk+-3.0; then source tools/gtk-dev/env.sh; fi; CGO_CFLAGS="-O0 -g0 -Wno-deprecated-declarations" go build -tags "gtk gtk_3_20" -o dvd-audio-gtk ./cmd/dvd-audio-gtk'

test:
	go test -race ./...
	go vet ./...

test-gtk:
	bash -c 'if ! pkg-config --exists gtk+-3.0; then source tools/gtk-dev/env.sh; fi; CGO_CFLAGS="-O0 -g0 -Wno-deprecated-declarations" go test -tags "gtk gtk_3_20" ./internal/desktop'

smoke-gtk: gtk
	G_DEBUG=fatal-criticals ./dvd-audio-gtk -lang en -smoke-test
	G_DEBUG=fatal-criticals ./dvd-audio-gtk -lang plt -smoke-test
	G_DEBUG=fatal-criticals ./dvd-audio-gtk -lang fr -smoke-test

# Native packaging: run on the destination OS with GTK development files installed.
package:
	bash scripts/package.sh

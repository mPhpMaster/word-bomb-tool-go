GOFLAGS ?= -mod=vendor
WINENV := GOOS=windows GOARCH=amd64

.PHONY: all gui cli test clean

all: gui cli

gui:
	$(WINENV) go build $(GOFLAGS) -ldflags "-H windowsgui -s -w" -o dist/WordBombGUI.exe ./cmd/wordbombgui

cli:
	$(WINENV) go build $(GOFLAGS) -ldflags "-s -w" -o dist/WordBombCLI.exe ./cmd/wordbombcli

test:
	go test ./...

clean:
	rm -rf dist

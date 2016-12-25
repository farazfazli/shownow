.PHONY: all
all:
	@bash -c "echo Building for Linux && GOOS=linux GOARCH=amd64 go build && echo Transferring && scp shownow tunnelvm: && echo Done"

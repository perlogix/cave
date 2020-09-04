BUILD_PATH=dist/$(shell date +%Y-%m-%d_%H)

build:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o cave .
	chmod +x cave

certs:
	openssl req -subj '/CN=*/' -subj '/CN=IP:localhost/' -newkey rsa:4096 -nodes -sha512 -x509 -days 3650 -nodes -out ssl/cave.crt -keyout ssl/cave.key


release: release-setup build-linux-x64 build-linux-x32

release-setup:
	mkdir -p $(BUILD_PATH)

build-linux-x64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o $(BUILD_PATH)/cave-linux-amd64 .
	chmod +x $(BUILD_PATH)/cave-linux-amd64

build-linux-x32:
	CGO_ENABLED=0 GOOS=linux GOARCH=386 go build -a -installsuffix cgo -o $(BUILD_PATH)/cave-linux-386 .
	chmod +x $(BUILD_PATH)/cave-linux-386

build-darwin-x64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -a -installsuffix cgo -o $(BUILD_PATH)/cave-macos-amd64 .
	chmod +x $(BUILD_PATH)/cave-macos-amd64

build-darwin-x32:
	CGO_ENABLED=0 GOOS=darwin GOARCH=386 go build -a -installsuffix cgo -o $(BUILD_PATH)/cave-macos-386 .
	chmod +x $(BUILD_PATH)/cave-macos-386

build-win-x64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o $(BUILD_PATH)/cave-win-amd64 .
	chmod +x $(BUILD_PATH)/cave-win-amd64

build-win-x32:
	CGO_ENABLED=0 GOOS=linux GOARCH=386 go build -a -installsuffix cgo -o $(BUILD_PATH)/cave-win-386 .
	chmod +x $(BUILD_PATH)/cave-win-386
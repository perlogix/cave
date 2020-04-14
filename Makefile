build:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bunker .
	chmod +x bunker

certs:
	openssl req -subj '/CN=*/' -subj '/CN=IP:localhost/' -newkey rsa:4096 -nodes -sha512 -x509 -days 3650 -nodes -out ssl/bunker.crt -keyout ssl/bunker.key
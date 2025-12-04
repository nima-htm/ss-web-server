install:
	go build -o ss-web-server
	sudo cp ss-web-server /opt

uninstall:
	sudo rm /opt/ss-web-server
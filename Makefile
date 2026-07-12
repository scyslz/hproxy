.PHONY: all build build-arm64 deploy clean

BINARY=hproxy
ARM_BINARY=hproxy-arm64
CONFIG=config.json

all: build

build:
	go build -o $(BINARY)

build-arm64:
	GOOS=linux GOARCH=arm64 go build -o $(ARM_BINARY)

deploy: build-arm64
	scp $(ARM_BINARY) root@192.168.100.1:/root/hproxy
	scp $(CONFIG) root@192.168.100.1:/root/config.json
	ssh root@192.168.100.1 "killall hproxy; cd /root && ./hproxy -config config.json &"

clean:
	rm -f $(BINARY) $(ARM_BINARY)
	rm -f /tmp/hproxy-api-cache.json

test:
	curl -k https://localhost:443/rules
	curl -k https://localhost:443/config
	curl -k https://localhost:443/reload

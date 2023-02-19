# Local Dev ===========================
client:
	JAM_ENV=local go run cmd/client/main.go 

web:
	JAM_ENV=local cd cmd/web/; go run main.go

server:
	JAM_ENV=local go run cmd/server/main.go

# Build ================================

clean:
	rm -rf jamsync-build && rm -rf jamsync-build.zip

zipself:
	mkdir -p ./jamsync-build/public && zip -r jamsync-build/public/jamsync-source.zip . -x .git/\*

protos:
	mkdir -p gen/go && protoc --proto_path=proto --go_out=gen/pb --go_opt=paths=source_relative --go-grpc_out=gen/pb --go-grpc_opt=paths=source_relative proto/*.proto

movewebassets:
	cp -R cmd/web/public jamsync-build/; cp -R cmd/web/template jamsync-build/; 

# Needed to be done locally since Mac requires signing binaries. Make sure you have signing env variables setup to do this.
buildclients:
	./scripts/buildclients.sh

# Run on server since ARM has some weirdness with cgo
buildservers:
	go build -o jamserver cmd/server/main.go && go build -o jamweb cmd/web/main.go

zipbuild:
	zip -r jamsync-build.zip jamsync-build/

# Make sure to setup hosts file to resolve ssh.prod.jamsync.dev to proper backend server.
uploadbuild:
	scp -i ~/jamsynckeypair.pem ./jamsync-build.zip ec2-user@ssh.prod.jamsync.dev:~/jamsync-build.zip

# Needed since make doesn't build same target twice and I didn't bother to find a better way
cleanbuild:
	rm -rf jamsync-build && rm -rf jamsync-build.zip

# Deploy ===============================

# Make sure to setup hosts file to resolve ssh.prod.jamsync.dev to proper backend server.
deploy:
	./scripts/deploy.sh

ssh:
	ssh -i ~/jamsynckeypair.pem ec2-user@ssh.prod.jamsync.dev

build: clean zipself protos movewebassets buildclients zipbuild uploadbuild cleanbuild deploy

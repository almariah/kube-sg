build:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o kube-sg .

.PHONY: docker
docker:
	docker build -t "abdullahalmariah/kube-sg:v0.0.1-alpha" .
	docker push "abdullahalmariah/kube-sg:v0.0.1-alpha"

.PHONY: clean
clean:
	rm -rf kube-sg

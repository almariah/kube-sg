CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o kube-sg .
docker build . -t abdullahalmariah/kube-sg:v0.0.1
docker push abdullahalmariah/kube-sg:v0.0.1

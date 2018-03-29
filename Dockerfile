FROM scratch
#RUN apk add --update ca-certificates

ADD ca-certificates.crt /etc/ssl/certs/
COPY kube-sg /kube-sg

ENTRYPOINT ["/kube-sg"]

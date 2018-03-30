FROM scratch

ADD misc/certs/ca-certificates.crt /etc/ssl/certs/
COPY kube-sg /kube-sg

ENTRYPOINT ["/kube-sg"]

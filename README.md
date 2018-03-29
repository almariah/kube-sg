# kube-sg
A controller that set ingress rules of AWS security groups using pod IP based on annotations.

A network policy `NetworkPolicy` is used to specify how groups of pods are allowed to communicate with each other and other network endpoints. An example `NetworkPolicy` might look like this:
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: test-network-policy
  namespace: default
spec:
  podSelector:
    matchLabels:
      role: db
  policyTypes:
  - Ingress
  ingress:
  - from:
    - ipBlock:
        cidr: 172.17.0.0/16
    ports:
    - protocol: TCP
      port: 6379
```

The previous network policy will allow access to pods with the label “role=db” from `172.17.0.0/16` on TCP port `6379`. The `cidr` could be any range of IP addresses (other pods, external endpoints, cloud resources).

In case of AWS, if pods IPs allocated from the VPC pool, then it is very useful to use network policy `ipBlock` from within the VPC pool to allow access to pods from other AWS resources.

### Accessing from pod to external resources (kube-sg)

Now, if access is required to other AWS resources from the pods within the smame VPC, then ingress rules should be added to security groups that is attached to these resources. The rules will be very dynamic if it is restrictive and allow only IP addresses of specific pods.

kube-sg will add, remove, update ingress rules form the specified security groups in pod templates based on annotations. To add ingress rules to specific security groups and ports:

Add an `sg.amazonaws.com/ingress` annotation to your pods with the rules that you want to add to security groups:
```yaml
...
kind: Pod
metadata:
annotations:
  sg.amazonaws.com/ingress: sg-1a2b3c4d:tcp:5000-5050/sg-1a2b3c4d:tcp:443/sg-1a2b3c4d:udp:7000-7005
```

When creating higher-level abstractions than pods, you need to pass the annotation in the pod template of the
resource spec.

```yaml
spec:
  template:
    metadata:
      annotations:
        sg.amazonaws.com/ingress: sg-1a2b3c4d:tcp:5000-5050/sg-1a2b3c4d:tcp:443/sg-1a2b3c4d:udp:7000-7005
```

## Installation

**Note: kube-sg assumes that pods IPs allocated from the VPC pool.**

To use kube-sg
```json
{
            "Action": [
                "ec2:AuthorizeSecurityGroupEgress",
                "ec2:AuthorizeSecurityGroupIngress",
                "ec2:DeleteSecurityGroup",
                "ec2:RevokeSecurityGroupEgress",
                "ec2:RevokeSecurityGroupIngress"
            ],
            "Resource": "*",
            "Effect": "Allow"
}
```


CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o kube-sg .

docker build . -t abdullahalmariah/kube-sg:v0.0.1

docker push abdullahalmariah/kube-sg:v0.0.1

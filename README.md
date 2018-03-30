# kube-sg
A controller that set ingress rules of AWS security groups using pod IP based on annotations.

## Overview

A [network policy](https://kubernetes.io/docs/concepts/services-networking/network-policies/) `NetworkPolicy` is used to specify how groups of pods are allowed to communicate with each other and other network endpoints. An example `NetworkPolicy` might look like this:
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

In case of AWS, if pods IPs allocated from the VPC pool ([amazon-vpc-cni-k8s](https://github.com/aws/amazon-vpc-cni-k8s)), then it is very useful to use network policy `ipBlock` from within the VPC pool to allow access to pods from other AWS resources.

### Accessing from pod to external resources (kube-sg)

Now, if access is required to other AWS resources from the pods within the smame VPC, then ingress rules should be added to security groups that is attached to these resources. The rules will be very dynamic if it is restrictive and allow only IP addresses of specific pods.

kube-sg will add, remove, update ingress rules form the specified security groups in pod templates based on annotations. To add ingress rules to specific security groups and ports:

Add `sg.amazonaws.com/<SECURITY_GROUP_ID>` annotation to your pods with the rules that you want to add to security groups:
```yaml
...
kind: Pod
metadata:
annotations:
  sg.amazonaws.com/sg-1a2b3c4d: tcp:5000-5050,tcp:443
  sg.amazonaws.com/sg-4a3b2c1d: udp:7000-7005
...
```

You can add comma-separated list of protocol and port combination to set multiple ingress rules for the same security group.

When creating higher-level abstractions than pods, you need to pass the annotations in the pod template of the
resource spec.

```yaml
...
spec:
  template:
    metadata:
      annotations:
        sg.amazonaws.com/sg-1a2b3c4d: tcp:5000-5050,tcp:443
        sg.amazonaws.com/sg-4a3b2c1d: udp:7000-7005
...
```

## Installation (kube-sg still experimental)

**Note: kube-sg assumes that pods IPs allocated from the same VPC pool where your resources are located or pods and resources are in the same network.**

To install kube-sg: first maek sure that you attach the following IAM policy to the master node:
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

Then deploy kube-sg controller:
```bash
kubectl apply -f misc/kube-sg.yaml
```

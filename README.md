# kube-sg
A controller that set ingress rules of AWS security groups using pod IP

## Usage

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

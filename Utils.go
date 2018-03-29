package main

import (
	"strings"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
)

func fromAnnotationsToRules(annotations, cidrIp, description string) map[string][]*ec2.IpPermission {

	rules := make(map[string][]*ec2.IpPermission)

	for _, rule := range strings.Split(annotations, ",") {
		var ipPermissions []*ec2.IpPermission
		s := strings.Split(rule, ":")
		if len(s) == 3 {
			ports := strings.Split(s[2], "-")
			if len(ports) == 2 {
				from, _ := strconv.ParseInt(ports[0], 10, 64)
        to, _ := strconv.ParseInt(ports[1], 10, 64)
				ipPermission := (&ec2.IpPermission{}).
						SetIpProtocol(s[1]).
						SetFromPort(from).
						SetToPort(to).
						SetIpRanges([]*ec2.IpRange{
								{
									CidrIp: aws.String(cidrIp),
									Description: aws.String(description),
								},
						})
				ipPermissions = append(ipPermissions, ipPermission)

				rules[s[0]] = ipPermissions
			} else if len(ports) == 1 {
				from, _ := strconv.ParseInt(ports[0], 10, 64)
				ipPermission := (&ec2.IpPermission{}).
						SetIpProtocol(s[1]).
						SetFromPort(from).
						SetToPort(from).
						SetIpRanges([]*ec2.IpRange{
								{
									CidrIp: aws.String(cidrIp),
									Description: aws.String(description),
								},
						})
				ipPermissions = append(ipPermissions, ipPermission)

				rules[s[0]] = ipPermissions
			}
		}
  }

	return rules
}

func getRegion() string {
	sess := session.New()
	identity, _ := ec2metadata.New(sess).GetInstanceIdentityDocument()
	region := identity.Region
	if region == "" {
		region = "us-east-1"
	}
	return region
}

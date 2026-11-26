package resolver

import (
	"context"
	"fmt"
	"net"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// Provides a interface for resolving an instance to a single ec2 instance.
type InstanceResolver interface {
	ResolveToInstanceId(ctx context.Context, client ec2.DescribeNetworkInterfacesAPIClient) ([]string, error)
}

// HostnameResolver attempts to resolve a fqdn or IP to a corresponding instance ID.
type HostnameResolver struct {
	addrs []string
}

func NewHostnameResolver(addrs []string) *HostnameResolver {
	return &HostnameResolver{
		addrs: addrs,
	}
}

func (hr *HostnameResolver) ResolveToInstanceId(ctx context.Context, client ec2.DescribeNetworkInterfacesAPIClient) (output []string, err error) {
	ips := make([]string, 1)
	for _, addr := range hr.addrs {
		ip, err := resolveToFirst(addr)
		if err != nil {
			return nil, fmt.Errorf("unable to resolve hostname to %v to ip", hr.addrs)
		}

		ips = append(ips, ip.String())
	}

	dniInput := &ec2.DescribeNetworkInterfacesInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("addresses.private-ip-address"),
				Values: ips,
			},
		},
	}

	// Fetch all the network interfaces described
	paginator := ec2.NewDescribeNetworkInterfacesPaginator(client, dniInput)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("could not describe network interfaces\n%w", err)
		}

		for _, nic := range page.NetworkInterfaces {
			output = append(output, *nic.Attachment.InstanceId)
		}
	}

	return output, nil
}

func resolveToFirst(addr string) (net.IP, error) {
	if ip := net.ParseIP(addr); ip != nil {
		return ip, nil
	} else if ips, err := net.LookupIP(addr); err == nil {
		if len(ips[0]) > 0 {
			return ips[0], nil
		}
	}

	return nil, fmt.Errorf("no IP address found for %s", addr)
}

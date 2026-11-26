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

// ResolveToInstanceId returns the IDs of the instances whose network interfaces
// have one of the resolver's addresses as a private IP. Each instance is listed
// once, in the order its interface was returned.
func (hr *HostnameResolver) ResolveToInstanceId(ctx context.Context, client ec2.DescribeNetworkInterfacesAPIClient) ([]string, error) {
	ips := make([]string, 0, len(hr.addrs))
	for _, addr := range hr.addrs {
		ip, err := resolveToFirst(addr)
		if err != nil {
			return nil, fmt.Errorf("unable to resolve %s to an IP address: %w", addr, err)
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

	var output []string
	seen := make(map[string]bool)

	// Fetch all the network interfaces described
	paginator := ec2.NewDescribeNetworkInterfacesPaginator(client, dniInput)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("could not describe network interfaces\n%w", err)
		}

		for _, nic := range page.NetworkInterfaces {
			// Interfaces owned by load balancers, NAT gateways, Lambda functions and
			// so on, or not attached at all, have no instance
			if nic.Attachment == nil || nic.Attachment.InstanceId == nil {
				continue
			}

			id := *nic.Attachment.InstanceId
			if !seen[id] {
				seen[id] = true
				output = append(output, id)
			}
		}
	}

	return output, nil
}

func resolveToFirst(addr string) (net.IP, error) {
	if ip := net.ParseIP(addr); ip != nil {
		return ip, nil
	}

	if ips, err := net.LookupIP(addr); err == nil && len(ips) > 0 {
		return ips[0], nil
	}

	return nil, fmt.Errorf("no IP address found for %s", addr)
}

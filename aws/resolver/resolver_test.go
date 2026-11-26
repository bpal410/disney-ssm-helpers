package resolver

import (
	"context"
	"net"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockedEC2 returns one page per entry in pages, and records the inputs it receives
type mockedEC2 struct {
	pages  []*ec2.DescribeNetworkInterfacesOutput
	inputs []*ec2.DescribeNetworkInterfacesInput
}

func (c *mockedEC2) DescribeNetworkInterfaces(_ context.Context, input *ec2.DescribeNetworkInterfacesInput, _ ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
	c.inputs = append(c.inputs, input)

	page := 0
	if input.NextToken != nil {
		page, _ = strconv.Atoi(*input.NextToken)
	}

	output := *c.pages[page]
	if page+1 < len(c.pages) {
		output.NextToken = aws.String(strconv.Itoa(page + 1))
	}
	return &output, nil
}

func attachedNIC(instanceID, ip string) ec2types.NetworkInterface {
	return ec2types.NetworkInterface{
		Attachment:       &ec2types.NetworkInterfaceAttachment{InstanceId: aws.String(instanceID)},
		PrivateIpAddress: aws.String(ip),
	}
}

var (
	exampleInstanceId       = "i-1234567890abcdef0"
	examplePrivateIpAddress = "127.0.0.1"
)

func TestHostnameResolver(t *testing.T) {
	ctx := context.Background()

	t.Run("test passed ip causes a short circuit", func(t *testing.T) {
		validIp, err := resolveToFirst(examplePrivateIpAddress)
		assert.Nil(t, err)
		assert.EqualValues(t, validIp, net.ParseIP(examplePrivateIpAddress))
	})

	t.Run("test passed hostname resolves to an IP", func(t *testing.T) {
		validIp, err := resolveToFirst("localhost")
		assert.Nil(t, err)
		assert.NotNil(t, validIp)
	})

	t.Run("test mock resolver returns a valid instance id", func(t *testing.T) {
		mockClient := &mockedEC2{pages: []*ec2.DescribeNetworkInterfacesOutput{
			{NetworkInterfaces: []ec2types.NetworkInterface{attachedNIC(exampleInstanceId, examplePrivateIpAddress)}},
		}}
		testResolver := NewHostnameResolver([]string{examplePrivateIpAddress})

		resp, err := testResolver.ResolveToInstanceId(ctx, mockClient)
		assert.Nil(t, err)
		assert.EqualValues(t, []string{exampleInstanceId}, resp)
	})

	t.Run("filter contains exactly the resolved IPs", func(t *testing.T) {
		mockClient := &mockedEC2{pages: []*ec2.DescribeNetworkInterfacesOutput{{}}}
		testResolver := NewHostnameResolver([]string{"10.0.0.1", "10.0.0.2"})

		_, err := testResolver.ResolveToInstanceId(ctx, mockClient)
		require.NoError(t, err)
		require.Len(t, mockClient.inputs, 1)
		require.Len(t, mockClient.inputs[0].Filters, 1)
		assert.Equal(t, []string{"10.0.0.1", "10.0.0.2"}, mockClient.inputs[0].Filters[0].Values)
	})

	t.Run("interfaces without an instance are skipped", func(t *testing.T) {
		mockClient := &mockedEC2{pages: []*ec2.DescribeNetworkInterfacesOutput{
			{NetworkInterfaces: []ec2types.NetworkInterface{
				// e.g. a load balancer interface: attached, but not to an instance
				{Attachment: &ec2types.NetworkInterfaceAttachment{}, PrivateIpAddress: aws.String("10.0.0.1")},
				// an unattached interface
				{PrivateIpAddress: aws.String("10.0.0.2")},
				attachedNIC("i-abc", "10.0.0.3"),
			}},
		}}
		testResolver := NewHostnameResolver([]string{"10.0.0.1", "10.0.0.2", "10.0.0.3"})

		resp, err := testResolver.ResolveToInstanceId(ctx, mockClient)
		assert.NoError(t, err)
		assert.Equal(t, []string{"i-abc"}, resp)
	})

	t.Run("results across pages are combined without duplicates", func(t *testing.T) {
		mockClient := &mockedEC2{pages: []*ec2.DescribeNetworkInterfacesOutput{
			{NetworkInterfaces: []ec2types.NetworkInterface{attachedNIC("i-abc", "10.0.0.1")}},
			{NetworkInterfaces: []ec2types.NetworkInterface{attachedNIC("i-def", "10.0.0.2"), attachedNIC("i-abc", "10.0.0.3")}},
		}}
		testResolver := NewHostnameResolver([]string{"10.0.0.1", "10.0.0.2", "10.0.0.3"})

		resp, err := testResolver.ResolveToInstanceId(ctx, mockClient)
		assert.NoError(t, err)
		assert.Equal(t, []string{"i-abc", "i-def"}, resp)
	})

	t.Run("unresolvable address is an error", func(t *testing.T) {
		testResolver := NewHostnameResolver([]string{"does-not-exist.invalid"})

		_, err := testResolver.ResolveToInstanceId(ctx, &mockedEC2{})
		assert.ErrorContains(t, err, "does-not-exist.invalid")
	})
}

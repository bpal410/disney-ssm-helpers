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

}

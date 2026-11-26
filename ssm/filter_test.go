package ssm

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/stretchr/testify/assert"
)

func TestAppendSSMFilter(t *testing.T) {
	assert := assert.New(t)

	// Create our filter slice
	filters := []ssmtypes.InstanceInformationStringFilter{
		{
			Key:    aws.String("PingStatus"),
			Values: []string{"Online"},
		},
	}

	// Create the filter to be appended
	secondFilter := ssmtypes.InstanceInformationStringFilter{
		Key:    aws.String("InstanceIds"),
		Values: []string{"i-123", "i-456"},
	}

	AppendSSMFilter(&filters, secondFilter)

	assert.Lenf(filters, 2, "Function returned slice of size %d, expected a size of 2", len(filters))
}

func Test_buildFilters(t *testing.T) {
	assert := assert.New(t)

	// Create our filter slice
	filters := []ssmtypes.InstanceInformationStringFilter{
		{
			Key:    aws.String("PingStatus"),
			Values: []string{"Online"},
		},
	}

	// Create the tags to be built and appended to the filter
	tags := map[string]string{
		"env": "dev",
		"foo": "bar",
	}

	buildFilters(tags, &filters)

	assert.Lenf(filters, 3, "Filter slice was built incorrectly, got %d filters, expected 3", len(filters))
}

func TestNewSSMInstanceFilter(t *testing.T) {
	assert := assert.New(t)

	filter := NewSSMInstanceFilter("InstanceIds", CommaSlice{"i-123", "i-456"})

	assert.Equal("InstanceIds", aws.ToString(filter.Key))
	assert.Equal([]string{"i-123", "i-456"}, filter.Values)
}

func TestNewSSMFilter(t *testing.T) {
	assert := assert.New(t)

	filter := newSSMFilter("filterkey", "foo", "bar", "baz")

	assert.Equal("filterkey", aws.ToString(filter.Key))
	assert.Equal([]string{"foo", "bar", "baz"}, filter.Values)
}

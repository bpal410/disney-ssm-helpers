package instance

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSet(t *testing.T) {
	assert := assert.New(t)

	is := InstanceSlice{}
	is.Set("i-123,i-234,i-345")

	assert.Lenf(is, 3, "Set method returned an incorrect number of results, got %d, expected 3", len(is))
}

func TestString(t *testing.T) {
	assert := assert.New(t)

	is := InstanceSlice{"foo", "bar", "baz"}

	assert.Equalf(is.String(), "[foo bar baz]", "String() returned an improper representation of the InstanceSlice object, got %v, expected '[foo bar baz]'", is.String())
}

func TestFormatString(t *testing.T) {
	assert := assert.New(t)

	ii := InstanceInfo{
		InstanceID: "i-123",
		Region:     "us-east-1",
		Profile:    "test",
		VpcId:      "vpc-1",
		Tags: map[string]string{
			"Name":    "web-1",
			"foo":     "oof",
			"bar":     "longvalue",
			"longkey": "baz",
		},
	}

	assert.Equal("i-123\tweb-1\tus-east-1\ttest\t", ii.FormatString())
	assert.Equal("i-123\tweb-1\tus-east-1\ttest\toof\tbaz\t", ii.FormatString("foo", "longkey"))
	assert.Equal("i-123\tweb-1\tus-east-1\ttest\tvpc-1\t", ii.FormatString("VpcId"))

	// Name is always shown, so asking for it again must not duplicate it
	assert.Equal("i-123\tweb-1\tus-east-1\ttest\toof\t", ii.FormatString("Name", "foo"))

	// Formatting must not add VpcId (or anything else) to the instance's tags
	assert.NotContains(ii.Tags, "VpcId")

	// Instances without a Name tag get an empty column
	unnamed := InstanceInfo{InstanceID: "i-456", Region: "us-east-1", Profile: "test"}
	assert.Equal("i-456\t\tus-east-1\ttest\t", unnamed.FormatString())
}

func TestFormatStringSlice(t *testing.T) {
	assert := assert.New(t)

	iis := InstanceInfoSafe{AllInstances: map[string]InstanceInfo{
		"i-123": {
			InstanceID: "i-123",
			Region:     "us-east-1",
			Profile:    "test",
			Tags:       map[string]string{"Name": "web-1", "env": "prod"},
		},
	}}

	got := iis.FormatStringSlice(NameTag, "Name", "env")

	// Header, one instance, and the trailing empty line
	assert.Len(got, 3)
	assert.Equal([]string{"Instance", "ID", "Name", "Region", "Profile", "env"}, strings.Fields(got[0]))
	assert.Equal([]string{"i-123", "web-1", "us-east-1", "test", "prod"}, strings.Fields(got[1]))
}

// rowIDs returns the instance ID of each row of FormatStringSlice output.
func rowIDs(lines []string) (ids []string) {
	for _, line := range lines[1 : len(lines)-1] {
		ids = append(ids, strings.Fields(line)[0])
	}
	return ids
}

func TestFormatStringSliceSorting(t *testing.T) {
	assert := assert.New(t)

	withTags := func(id string, tags map[string]string) InstanceInfo {
		return InstanceInfo{InstanceID: id, Region: "us-east-1", Profile: "test", Tags: tags}
	}

	iis := InstanceInfoSafe{AllInstances: map[string]InstanceInfo{}}
	for _, ii := range []InstanceInfo{
		withTags("i-6", map[string]string{"env": "dev"}),
		withTags("i-5", map[string]string{"Name": "web-2", "env": "prod"}),
		withTags("i-4", map[string]string{"Name": "Web-1"}),
		withTags("i-3", map[string]string{"Name": "db", "env": "Prod"}),
		withTags("i-2", nil),
		withTags("i-1", map[string]string{"Name": "web-2", "env": "dev"}),
	} {
		iis.AllInstances[ii.InstanceID] = ii
	}

	t.Run("by Name", func(t *testing.T) {
		lines := iis.FormatStringSlice(NameTag)

		// db, Web-1 (case-insensitive), web-2 x2 by ID, then unnamed by ID
		assert.Equal([]string{"i-3", "i-4", "i-1", "i-5", "i-2", "i-6"}, rowIDs(lines))
		assert.Equal([]string{"Instance", "ID", "Name", "Region", "Profile"}, strings.Fields(lines[0]))
	})

	t.Run("by another tag adds it as a column", func(t *testing.T) {
		lines := iis.FormatStringSlice("env")

		// dev x2 by ID, prod x2 (case-insensitive) by ID, then untagged by ID
		assert.Equal([]string{"i-1", "i-6", "i-3", "i-5", "i-2", "i-4"}, rowIDs(lines))
		assert.Equal([]string{"Instance", "ID", "Name", "Region", "Profile", "env"}, strings.Fields(lines[0]))
	})

	t.Run("by a tag already shown does not duplicate it", func(t *testing.T) {
		lines := iis.FormatStringSlice("env", "foo", "env")

		assert.Equal([]string{"Instance", "ID", "Name", "Region", "Profile", "foo", "env"}, strings.Fields(lines[0]))
	})

	t.Run("empty sort tag sorts by instance ID", func(t *testing.T) {
		lines := iis.FormatStringSlice("")

		assert.Equal([]string{"i-1", "i-2", "i-3", "i-4", "i-5", "i-6"}, rowIDs(lines))
	})
}

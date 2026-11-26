package instance

import (
	"bytes"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/bpal410/ssm-helpers/util"
)

// InstanceInfoSafe allows for concurrent-safe access to a map of InstanceInfo data
type InstanceInfoSafe struct {
	sync.Mutex
	AllInstances map[string]InstanceInfo
}

// InstanceInfo is used to store information, including EC2 tags, about a particular instance
type InstanceInfo struct {
	InstanceID string
	Region     string
	Profile    string
	VpcId      string
	Tags       map[string]string
}

// NameTag is the EC2 tag shown as a column after the instance ID in every
// instance listing.
const NameTag = "Name"

// FormatStringSlice is used to return a strings preformatted to the correct width for selection prompts.
// The first element is the header row. Instance ID, Name, Region and Profile
// are always included; includeFields adds further tag or attribute columns.
//
// Rows are sorted by the sortTag value (case-insensitive), with instances
// missing that tag last and instance ID breaking ties; an empty sortTag sorts
// by instance ID only. If sortTag is not already a column it is added as one,
// so the order is visible.
func (i *InstanceInfoSafe) FormatStringSlice(sortTag string, includeFields ...string) (outSlice []string) {
	stringBuffer := new(bytes.Buffer)

	// Set up our tabwriter to nicely space our output
	tw := tabwriter.NewWriter(stringBuffer, 5, 4, 2, ' ', 0)

	columns := extraColumns(sortTag, includeFields)

	// Set up our header string
	headerString := "Instance ID\t" + NameTag + "\tRegion\tProfile\t"
	for _, fieldName := range columns {
		headerString = fmt.Sprintf("%s%s\t", headerString, fieldName)
	}
	fmt.Fprintf(tw, "%s\n", headerString)

	for _, v := range i.sortedBy(sortTag) {
		fmt.Fprintf(tw, "%v\n", v.FormatString(columns...))
	}

	tw.Flush()

	return strings.Split(stringBuffer.String(), "\n")
}

// sortedBy returns the instances ordered by the value of the given tag
// (case-insensitive), with instances missing it last and instance ID breaking
// ties.
func (i *InstanceInfoSafe) sortedBy(tag string) []InstanceInfo {
	instances := make([]InstanceInfo, 0, len(i.AllInstances))
	for _, v := range i.AllInstances {
		instances = append(instances, v)
	}

	sort.Slice(instances, func(a, b int) bool {
		valueA := strings.ToLower(instances[a].field(tag))
		valueB := strings.ToLower(instances[b].field(tag))

		if valueA != valueB {
			if valueA == "" || valueB == "" {
				return valueB == "" // instances without the tag go last
			}
			return valueA < valueB
		}
		return instances[a].InstanceID < instances[b].InstanceID
	})

	return instances
}

// FormatString returns a string with various information about a given instance
func (i *InstanceInfo) FormatString(includeFields ...string) string {
	// Formatted string will always contain at least base info
	formattedString := fmt.Sprintf("%s\t%s\t%s\t%s\t", i.InstanceID, i.Tags[NameTag], i.Region, i.Profile)

	for _, v := range extraColumns("", includeFields) {
		formattedString = fmt.Sprintf("%s%s\t", formattedString, i.field(v))
	}

	return formattedString
}

// field returns the value of the named tag, or of the VpcId attribute.
func (i *InstanceInfo) field(name string) string {
	if name == "VpcId" {
		return i.VpcId
	}
	return i.Tags[name]
}

// extraColumns returns the columns to show after the fixed ones: sortTag
// (if it is not already shown) followed by includeFields, without the Name
// tag, which is always displayed.
func extraColumns(sortTag string, includeFields []string) []string {
	var out []string
	if sortTag != "" && sortTag != NameTag && !slices.Contains(includeFields, sortTag) {
		out = append(out, sortTag)
	}

	for _, f := range includeFields {
		if f != NameTag {
			out = append(out, f)
		}
	}
	return out
}

// InstanceSlice is the type of object passed to flag.Var() that holds AWS instance IDs
type InstanceSlice []string

// String() returns the string representation of an *InstanceSlice object
func (i *InstanceSlice) String() string {
	return fmt.Sprintf("%s", *i)
}

// Set splits the provided comma-delimited list of instance IDs into an InstanceSlice, then sets the value of the caller to the new slice
func (i *InstanceSlice) Set(value string) error {
	list := util.CommaSplit(value)
	for _, v := range list {
		*i = append(*i, v)
	}
	return nil
}

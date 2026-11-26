package ssm

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// AppendSSMFilter appends a single ssmtypes.InstanceInformationStringFilter to an existing slice of filters
func AppendSSMFilter(filters *[]ssmtypes.InstanceInformationStringFilter, filterToAdd ssmtypes.InstanceInformationStringFilter) {
	*filters = append(*filters, filterToAdd)
}

// buildFilters takes a map of tags (in key[value] format) and converts them into
// ssmtypes.InstanceInformationStringFilter values, appending them to the provided slice
func buildFilters(commonTags map[string]string, filtersList *[]ssmtypes.InstanceInformationStringFilter) {
	for k, v := range commonTags {
		*filtersList = append(*filtersList, newSSMFilter("tag:"+k, v))
	}
}

// NewSSMInstanceFilter takes a name and any number of instance IDs to create an ssmtypes.InstanceInformationStringFilter
// that can be used to select specific SSM-managed instances instead of solely searching based on tags
func NewSSMInstanceFilter(name string, values CommaSlice) ssmtypes.InstanceInformationStringFilter {
	return ssmtypes.InstanceInformationStringFilter{
		Key:    aws.String(name),
		Values: append([]string(nil), values...),
	}
}

// newSSMFilter takes a name and any number of string values and returns an ssmtypes.InstanceInformationStringFilter
func newSSMFilter(name string, values ...string) ssmtypes.InstanceInformationStringFilter {
	return ssmtypes.InstanceInformationStringFilter{
		Key:    aws.String(name),
		Values: values,
	}
}

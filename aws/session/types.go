package session

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/sirupsen/logrus"
)

// Session is an initialized AWS configuration for a target profile and region
type Session struct {
	Logger      *logrus.Logger
	Config      aws.Config
	ProfileName string
}

// Region returns the region this session's API calls are made in
func (s *Session) Region() string {
	return s.Config.Region
}

// Pool contains a set of session configurations for the target profiles and regions
type Pool struct {
	Sessions map[string]*Session
}

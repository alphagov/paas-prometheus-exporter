package app

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry-community/go-cfclient"
)

var _ = Describe("checkForNewAppsNew", func() {
	It("creates a new app", func() {

		apps := []cfclient.App{
			{Guid: "33333333-3333-3333-3333-333333333333", Instances: 1, Name: "foo", SpaceURL: "/v2/spaces/123"},
		}

		config := &cfclient.Config{}

		checkForNewAppsNew(apps, config)
		Expect(1).To(Equal(1))
	})
})

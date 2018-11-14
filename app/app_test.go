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

// test we hope to write

// when an app is created we crate an app AppWatcher
// when it's deleted we delete the appWatcher
// when an app is renamed we delete the old watcher and create a new one
// when an app is scaled we tell the app watcher

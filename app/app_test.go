package app_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/ghttp"

	"github.com/cloudfoundry-community/go-cfclient"

	"github.com/alphagov/paas-prometheus-exporter/app"
	"github.com/alphagov/paas-prometheus-exporter/app/mocks"
)

var _ = Describe("CheckForNewAppsNew", func() {

	var fakeClient mocks.FakeCFClient

	BeforeEach(func() {
		fakeClient = mocks.FakeCFClient{}
	})

	It("creates a new app", func() {

		apps := []cfclient.App{
			{Guid: "33333333-3333-3333-3333-333333333333", Instances: 1, Name: "foo", SpaceURL: "/v2/spaces/123"},
		}

		config := &cfclient.Config{}

		app.CheckForNewAppsNew(apps, config)

		// Test that a new watcher is created.
		// Mock out `events.NewAppWatcher` to capture arguments passed to it.
	})

	It("does a thing", func() {
		client := cfclient.NewClient(&cfclient.Config{})
		fakeClient
		apps, err := app.Test(client)

		Expect(err).ToNot(HaveOccurred())
		Expect(len(apps)).To(Equal(2))
	})
})

// test we hope to write

// when an app is created we create an app AppWatcher
// when it's deleted we delete the appWatcher
// when an app is renamed we delete the old watcher and create a new one
// when an app is scaled we tell the app watcher

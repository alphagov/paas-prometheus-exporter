package exporter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	//. "github.com/onsi/gomega/ghttp"

	"github.com/cloudfoundry-community/go-cfclient"

	"github.com/alphagov/paas-prometheus-exporter/exporter"
	"github.com/alphagov/paas-prometheus-exporter/exporter/mocks"
)

var _ = Describe("CheckForNewApps", func() {

	var fakeClient *mocks.FakeCFClient

	BeforeEach(func() {
		fakeClient = &mocks.FakeCFClient{}
	})

	It("creates a new app", func() {
		fakeClient.ListAppsByQueryReturns([]cfclient.App{
			{Guid: "33333333-3333-3333-3333-333333333333", Instances: 1, Name: "foo", SpaceURL: "/v2/spaces/123"},
		}, nil )

		config := &cfclient.Config{}

		e := exporter.New(fakeClient, config)

		e.CheckForNewApps()

		Expect(e.WatcherCount()).To(Equal(1))
	})

})

// test we hope to write

// when an app is created we create an app AppWatcher
// when it's deleted we delete the appWatcher
// when an app is renamed we delete the old watcher and create a new one
// when an app is scaled we tell the app watcher

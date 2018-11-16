package exporter_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry-community/go-cfclient"

	"github.com/alphagov/paas-prometheus-exporter/exporter"
	"github.com/alphagov/paas-prometheus-exporter/exporter/mocks"
)

var _ = Describe("CheckForNewApps", func() {

	var fakeClient *mocks.FakeCFClient
	var fakeWatcherManager *mocks.FakeWatcherManager

	BeforeEach(func() {
		fakeClient = &mocks.FakeCFClient{}
		fakeWatcherManager = &mocks.FakeWatcherManager{}
	})

	It("creates a new app", func() {
		fakeClient.ListAppsByQueryReturns([]cfclient.App{
			{Guid: "33333333-3333-3333-3333-333333333333", Instances: 1, Name: "foo", SpaceURL: "/v2/spaces/123"},
		}, nil)

		e := exporter.New(fakeClient, fakeWatcherManager)

		go e.Start(100 * time.Millisecond)

		Eventually(fakeWatcherManager.AddWatcherCallCount).Should(Equal(1))
	})

	It("deletes an AppWatcher when an app is deleted", func() {
		fakeClient.ListAppsByQueryReturnsOnCall(0, []cfclient.App{
			{Guid: "33333333-3333-3333-3333-333333333333", Instances: 1, Name: "foo", SpaceURL: "/v2/spaces/123"},
		}, nil)
		fakeClient.ListAppsByQueryReturns([]cfclient.App{}, nil)

		e := exporter.New(fakeClient, fakeWatcherManager)

		go e.Start(100 * time.Millisecond)

		Eventually(fakeWatcherManager.AddWatcherCallCount).Should(Equal(1))
		Eventually(fakeWatcherManager.DeleteWatcherCallCount).Should(Equal(1))
	})

	XIt("deletes and recreates an AppWatcher when an app is renamed", func() {

	})
	XIt("updates an AppWatcher when an app changes size", func() {

	})
})

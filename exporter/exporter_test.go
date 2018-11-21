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
			{Guid: "33333333-3333-3333-3333-333333333333", Instances: 1, Name: "foo", SpaceURL: "/v2/spaces/123", State: "STARTED"},
		}, nil)

		e := exporter.New(fakeClient, fakeWatcherManager)

		go e.Start(100 * time.Millisecond)

		Eventually(fakeWatcherManager.AddWatcherCallCount).Should(Equal(1))
	})

	It("does not create a new appWatcher if the app state is stopped", func() {
		fakeClient.ListAppsByQueryReturns([]cfclient.App{
			{Guid: "33333333-3333-3333-3333-333333333333", Instances: 1, Name: "foo", SpaceURL: "/v2/spaces/123", State: "STOPPED"},
		}, nil)

		e := exporter.New(fakeClient, fakeWatcherManager)

		go e.Start(100 * time.Millisecond)

		Eventually(fakeWatcherManager.AddWatcherCallCount).Should(Equal(0))
	})

	It("deletes an AppWatcher when an app is deleted", func() {
		fakeClient.ListAppsByQueryReturnsOnCall(0, []cfclient.App{
			{Guid: "33333333-3333-3333-3333-333333333333", Instances: 1, Name: "foo", SpaceURL: "/v2/spaces/123", State: "STARTED"},
		}, nil)
		fakeClient.ListAppsByQueryReturns([]cfclient.App{}, nil)

		e := exporter.New(fakeClient, fakeWatcherManager)

		go e.Start(100 * time.Millisecond)

		Eventually(fakeWatcherManager.AddWatcherCallCount).Should(Equal(1))
		Eventually(fakeWatcherManager.DeleteWatcherCallCount).Should(Equal(1))
	})

	It("deletes an AppWatcher when an app is stopped", func() {
		fakeClient.ListAppsByQueryReturnsOnCall(0, []cfclient.App{
			{Guid: "11111111-11111-11111-1111-111-11-1-1-1", Instances: 1, Name: "foo", SpaceURL: "/v2/spaces/123", State: "STARTED"},
		}, nil)
		fakeClient.ListAppsByQueryReturns([]cfclient.App{
			{Guid: "11111111-11111-11111-1111-111-11-1-1-1", Instances: 0, Name: "foo", SpaceURL: "/v2/spaces/123", State: "STOPPED"},
		}, nil)

		e := exporter.New(fakeClient, fakeWatcherManager)

		go e.Start(100 * time.Millisecond)

		Eventually(fakeWatcherManager.AddWatcherCallCount).Should(Equal(1))
		Eventually(fakeWatcherManager.DeleteWatcherCallCount).Should(Equal(1))

		// Check that we don't try and delete it a second time in the next run of `checkForNewApps`
		Consistently(fakeWatcherManager.DeleteWatcherCallCount, 200 * time.Millisecond).Should(Equal(1))
	})

	It("deletes and recreates an AppWatcher when an app is renamed", func() {
		fakeClient.ListAppsByQueryReturnsOnCall(0, []cfclient.App{
			{Guid: "33333333-3333-3333-3333-333333333333", Instances: 1, Name: "foo", SpaceURL: "/v2/spaces/123", State: "STARTED"},
		}, nil)
		fakeClient.ListAppsByQueryReturns([]cfclient.App{
			{Guid: "33333333-3333-3333-3333-333333333333", Instances: 1, Name: "bar", SpaceURL: "/v2/spaces/123", State: "STARTED"},
		}, nil)

		e := exporter.New(fakeClient, fakeWatcherManager)

		go e.Start(100 * time.Millisecond)

		Eventually(fakeWatcherManager.AddWatcherCallCount).Should(Equal(2))
		Eventually(fakeWatcherManager.DeleteWatcherCallCount).Should(Equal(1))
	})

	It("updates an AppWatcher when an app changes size", func() {
		fakeClient.ListAppsByQueryReturnsOnCall(0, []cfclient.App{
			{Guid: "33333333-3333-3333-3333-333333333333", Instances: 1, Name: "foo", SpaceURL: "/v2/spaces/123", State: "STARTED"},
		}, nil)
		fakeClient.ListAppsByQueryReturns([]cfclient.App{
			{Guid: "33333333-3333-3333-3333-333333333333", Instances: 2, Name: "foo", SpaceURL: "/v2/spaces/123", State: "STARTED"},
		}, nil)

		e := exporter.New(fakeClient, fakeWatcherManager)

		go e.Start(100 * time.Millisecond)

		Eventually(fakeWatcherManager.AddWatcherCallCount).Should(Equal(1))
		Eventually(fakeWatcherManager.UpdateAppInstancesCallCount).Should(Equal(1))

		guid, instances := fakeWatcherManager.UpdateAppInstancesArgsForCall(0)
		Expect(guid).To(Equal("33333333-3333-3333-3333-333333333333"))
		Expect(instances).To(Equal(2))
	})
})

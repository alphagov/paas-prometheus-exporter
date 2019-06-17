package cf_test

import (
	"github.com/alphagov/paas-prometheus-exporter/cf"
	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client", func() {
	BeforeSuite(func() {
		httpmock.Activate()
	})

	BeforeEach(func() {
		httpmock.Reset()

		httpmock.RegisterResponder("GET", "http://api.bosh-lite.com/v2/info", httpmock.NewStringResponder(200, `{}`))
		httpmock.RegisterResponder("POST", "/oauth/token", httpmock.NewStringResponder(200, `{"access_token": "some-access-token"}`))
	})

	AfterSuite(func() {
		httpmock.DeactivateAndReset()
	})

	It("should list apps with spaces and orgs", func() {
		config := cfclient.DefaultConfig()
		config.HttpClient.Transport = httpmock.DefaultTransport

		client, err := cf.NewClient(config, "")
		Expect(err).NotTo(HaveOccurred())

		httpmock.RegisterResponder(
			"GET",
			"http://api.bosh-lite.com/v2/organizations?",
			httpmock.NewStringResponder(200, `{"resources":[{"metadata":{"guid":"some-org-guid"}}]}`),
		)

		httpmock.RegisterResponder(
			"GET",
			"http://api.bosh-lite.com/v2/spaces?",
			httpmock.NewStringResponder(200, `{"resources":[{
				"metadata":{"guid":"some-space-guid"},
				"entity":{"organization_guid":"some-org-guid"}
			}]}`),
		)

		httpmock.RegisterResponder(
			"GET",
			"http://api.bosh-lite.com/v2/apps?",
			httpmock.NewStringResponder(200, `{"resources":[{"entity":{"space_guid":"some-space-guid","organization_guid":"some-org-guid"}}]}`),
		)

		apps, err := client.ListAppsWithSpaceAndOrg()
		Expect(err).NotTo(HaveOccurred())
		Expect(apps).To(HaveLen(1))
		space := apps[0].SpaceData.Entity
		Expect(space.Guid).To(Equal("some-space-guid"))
		org := space.OrgData.Entity
		Expect(org.Guid).To(Equal("some-org-guid"))
	})

	It("should list services with spaces and orgs", func() {
		config := cfclient.DefaultConfig()
		config.HttpClient.Transport = httpmock.DefaultTransport

		client, err := cf.NewClient(config, "")
		Expect(err).NotTo(HaveOccurred())

		httpmock.RegisterResponder(
			"GET",
			"http://api.bosh-lite.com/v2/organizations?",
			httpmock.NewStringResponder(200, `{"resources":[{"metadata":{"guid":"some-org-guid"}}]}`),
		)

		httpmock.RegisterResponder(
			"GET",
			"http://api.bosh-lite.com/v2/spaces?",
			httpmock.NewStringResponder(200, `{"resources":[{
				"metadata":{"guid":"some-space-guid"},
				"entity":{"organization_guid":"some-org-guid"}
			}]}`),
		)
		httpmock.RegisterResponder(
			"GET",
			"http://api.bosh-lite.com/v2/service_instances?",
			httpmock.NewStringResponder(200, `{"resources":[{"entity": {"space_guid":"some-space-guid"}}]}`),
		)

		services, err := client.ListServicesWithSpaceAndOrg()
		Expect(err).NotTo(HaveOccurred())
		Expect(services).To(HaveLen(1))
		space := services[0].SpaceData.Entity
		Expect(space.Guid).To(Equal("some-space-guid"))
		org := space.OrgData.Entity
		Expect(org.Guid).To(Equal("some-org-guid"))
	})
})

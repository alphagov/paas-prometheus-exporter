package main

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Building the HTTP server", func() {
	var (
		port        int
		username    string
		password    string
		server      *http.Server
		promHandler *ghttp.Server
	)

	BeforeEach(func() {
		port = 8080
		username, password = "", ""

		promHandler = ghttp.NewUnstartedServer()
		promHandler.AllowUnhandledRequests = true
		promHandler.UnhandledRequestStatusCode = http.StatusOK
	})

	JustBeforeEach(func() {
		server = buildHTTPServer(port, promHandler, username, password)
	})

	It("constructs the server to listen on the given port", func() {
		Expect(server.Addr).To(Equal(":8080"))
	})

	It("passes /metrics requests to the given handler", func() {
		req := httptest.NewRequest("GET", "http://www.example.com/metrics", nil)
		resp := httptest.NewRecorder()
		server.Handler.ServeHTTP(resp, req)

		Expect(resp.Code).To(Equal(http.StatusOK))
		Expect(promHandler.ReceivedRequests()).To(HaveLen(1))
	})

	It("returns a 404 for other paths", func() {
		req := httptest.NewRequest("GET", "http://www.example.com/", nil)
		resp := httptest.NewRecorder()
		server.Handler.ServeHTTP(resp, req)

		Expect(resp.Code).To(Equal(http.StatusNotFound))
		Expect(promHandler.ReceivedRequests()).To(HaveLen(0))
	})

	Context("with basic auth", func() {
		BeforeEach(func() {
			username = "user"
			password = "secret"
		})

		It("rejects requests without basic auth", func() {
			req := httptest.NewRequest("GET", "http://www.example.com/metrics", nil)
			req.SetBasicAuth(username, "not-the-password")
			resp := httptest.NewRecorder()
			server.Handler.ServeHTTP(resp, req)

			Expect(resp.Code).To(Equal(http.StatusUnauthorized))
			Expect(promHandler.ReceivedRequests()).To(HaveLen(0))
		})

		It("allows valid requests through", func() {
			req := httptest.NewRequest("GET", "http://www.example.com/metrics", nil)
			req.SetBasicAuth(username, password)
			resp := httptest.NewRecorder()
			server.Handler.ServeHTTP(resp, req)

			Expect(resp.Code).To(Equal(http.StatusOK))
			Expect(promHandler.ReceivedRequests()).To(HaveLen(1))
		})
	})
})

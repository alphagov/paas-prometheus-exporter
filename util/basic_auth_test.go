package util_test

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	"github.com/alphagov/paas-prometheus-exporter/util"
)

var _ = Describe("Basic auth handler", func() {
	const (
		username = "user"
		password = "secret"
		realm    = "auth"
	)

	var (
		backend     *ghttp.Server
		authHandler http.Handler
		req         *http.Request
		response    *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		backend = ghttp.NewUnstartedServer()
		backend.AllowUnhandledRequests = true
		backend.UnhandledRequestStatusCode = http.StatusOK
		authHandler = util.BasicAuthHandler(username, password, realm, backend)
		req = httptest.NewRequest("GET", "http://www.example.com/", nil)
		response = httptest.NewRecorder()
	})

	JustBeforeEach(func() {
		authHandler.ServeHTTP(response, req)
	})

	Context("with the correct username and password", func() {
		BeforeEach(func() {
			req.SetBasicAuth(username, password)
		})

		It("should pass the request to the backend", func() {
			Expect(response.Code).To(Equal(http.StatusOK))

			Expect(backend.ReceivedRequests()).To(HaveLen(1))
		})
	})

	Context("with invalid credentials", func() {
		BeforeEach(func() {
			req.SetBasicAuth(username, "not the password")
		})

		It("returns a 401 Unauthorized", func() {
			Expect(response.Code).To(Equal(http.StatusUnauthorized))
			Expect(response.Header().Get("WWW-Authenticate")).To(Equal(`Basic realm="auth"`))
		})

		It("does not make a request to the backend", func() {
			Expect(backend.ReceivedRequests()).To(HaveLen(0))
		})
	})
})

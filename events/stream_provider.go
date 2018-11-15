package events

import (
	"crypto/tls"

	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/cloudfoundry/noaa/consumer"
	sonde_events "github.com/cloudfoundry/sonde-go/events"
)

type AppStreamProvider interface {
	OpenStreamFor(appGuid string) (<-chan *sonde_events.Envelope, <-chan error)
}

type DopplerAppStreamProvider struct {
	config             *cfclient.Config
	cfClient           *cfclient.Client
}

func (m *DopplerAppStreamProvider) OpenStreamFor(appGuid string) ( <-chan *sonde_events.Envelope, <-chan error) {
	err := m.authenticate()
	if err != nil {
		errs := make(chan error,1)
		errs <- err 
		return nil, errs
	}
	tlsConfig := tls.Config{InsecureSkipVerify: false} // TODO: is this needed?
	conn := consumer.New(m.cfClient.Endpoint.DopplerEndpoint, &tlsConfig, nil)
	conn.RefreshTokenFrom(m)
	defer conn.Close()

	authToken, err := m.cfClient.GetToken()
	if err != nil {
		errs := make(chan error,1)
		errs <- err 
		return nil, errs
	}

	msgs, errs := conn.Stream(appGuid, authToken)
	return msgs, errs
}

// RefreshAuthToken satisfies the `consumer.TokenRefresher` interface.
func (m *DopplerAppStreamProvider) RefreshAuthToken() (token string, authError error) {
	token, err := m.cfClient.GetToken()
	if err != nil {
		err := m.authenticate()

		if err != nil {
			return "", err
		}

		return m.cfClient.GetToken()
	}

	return token, nil
}

func (m *DopplerAppStreamProvider) authenticate() (err error) {
	client, err := cfclient.NewClient(m.config)
	if err != nil {
		return err
	}

	m.cfClient = client
	return nil
}

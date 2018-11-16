package events

import (
	"crypto/tls"

	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/cloudfoundry/noaa/consumer"
	sonde_events "github.com/cloudfoundry/sonde-go/events"
)

//go:generate counterfeiter -o mocks/stream_provider.go . AppStreamProvider
type AppStreamProvider interface {
	OpenStreamFor(appGuid string) (<-chan *sonde_events.Envelope, <-chan error)
	Close() error
}

type closable interface {
	Close() error
}

type DopplerAppStreamProvider struct {
	Config   *cfclient.Config
	cfClient *cfclient.Client
	conn     closable
}

func (m *DopplerAppStreamProvider) OpenStreamFor(appGuid string) (<-chan *sonde_events.Envelope, <-chan error) {
	err := m.authenticate()
	if err != nil {
		errs := make(chan error,1)
		errs <- err
		return nil, errs
	}
	tlsConfig := tls.Config{InsecureSkipVerify: false} // TODO: is this needed?
	conn := consumer.New(m.cfClient.Endpoint.DopplerEndpoint, &tlsConfig, nil)
	m.conn = conn
	conn.RefreshTokenFrom(m)

	authToken, err := m.cfClient.GetToken()
	if err != nil {
		errs := make(chan error,1)
		errs <- err
		return nil, errs
	}

	msgs, errs := conn.Stream(appGuid, authToken)
	return msgs, errs
}

func (m *DopplerAppStreamProvider) Close() error {
	return m.conn.Close()
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
	client, err := cfclient.NewClient(m.Config)
	if err != nil {
		return err
	}

	m.cfClient = client
	return nil
}

package cf

import (
	"github.com/cloudfoundry/noaa/consumer"
	sonde_events "github.com/cloudfoundry/sonde-go/events"
)

//go:generate counterfeiter -o mocks/stream_provider.go . AppStreamProvider
type AppStreamProvider interface {
	Start() (<-chan *sonde_events.Envelope, <-chan error)
	Close() error
}

type DopplerAppStreamProvider struct {
	appGUID  string
	consumer *consumer.Consumer
	client   Client
}

func NewDopplerAppStreamProvider(client Client, appGUID string) *DopplerAppStreamProvider {
	consumer := consumer.New(client.DopplerEndpoint(), nil, nil)
	consumer.RefreshTokenFrom(client)

	return &DopplerAppStreamProvider{
		appGUID:  appGUID,
		consumer: consumer,
		client:   client,
	}
}

func (d *DopplerAppStreamProvider) Start() (<-chan *sonde_events.Envelope, <-chan error) {
	authToken, err := d.client.GetToken()
	if err != nil {
		errs := make(chan error, 1)
		errs <- err
		return nil, errs
	}

	msgs, errs := d.consumer.Stream(d.appGUID, authToken)
	return msgs, errs
}

func (d *DopplerAppStreamProvider) Close() error {
	return d.consumer.Close()
}

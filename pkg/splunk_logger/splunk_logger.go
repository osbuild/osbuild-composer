package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

const (
	PayloadsChannelSize = 1000
	// in seconds, how often batched events should be sent
	SendFrequency = 5
)

type SplunkLogger struct {
	client   *http.Client
	url      string
	token    string
	source   string
	hostname string

	payloads chan *SplunkPayload
}

type SplunkPayload struct {
	// splunk expects unix time in seconds
	Time  int64       `json:"time"`
	Host  string      `json:"host"`
	Event SplunkEvent `json:"event"`
}

type SplunkEvent struct {
	Message string `json:"message"`
	Ident   string `json:"ident"`
	Host    string `json:"host"`
}

func NewSplunkLogger(url, token, source, hostname string) *SplunkLogger {
	sl := &SplunkLogger{
		client:   retryablehttp.NewClient().StandardClient(),
		url:      url,
		token:    token,
		source:   source,
		hostname: hostname,
	}

	ticker := time.NewTicker(time.Second * SendFrequency)
	sl.payloads = make(chan *SplunkPayload, PayloadsChannelSize)

	go sl.flushPayloads(ticker.C)

	return sl
}

func (sl *SplunkLogger) flushPayloads(ticker <-chan time.Time) {
	var payloads []*SplunkPayload
	for {
		select {
		case p := <-sl.payloads:
			if p != nil {
				payloads = append(payloads, p)
			}
			if len(payloads) == PayloadsChannelSize {
				err := sl.SendPayloads(payloads)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Splunk logger unable to send payloads: %v", err)
				}
				payloads = nil
			}
		case <-ticker:
			err := sl.SendPayloads(payloads)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Splunk logger unable to send payloads: %v", err)
			}
			payloads = nil
		}
	}
}

func (sl *SplunkLogger) SendPayloads(payloads []*SplunkPayload) error {
	if len(payloads) == 0 {
		return nil
	}

	buf := bytes.NewBuffer(nil)
	for _, pl := range payloads {
		b, err := json.Marshal(pl)
		if err != nil {
			return err
		}

		_, err = buf.Write(b)
		if err != nil {
			return err
		}
	}

	req, err := http.NewRequest("POST", sl.url, bytes.NewReader(buf.Bytes()))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Splunk %s", sl.token))

	res, err := sl.client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Unable to close response body when sending payloads")
		}
	}()

	if res.StatusCode != http.StatusOK {
		buf := bytes.Buffer{}
		_, err = buf.ReadFrom(res.Body)
		if err != nil {
			return fmt.Errorf("Error forwarding to splunk: parsing response failed: %v", err)
		}
		return fmt.Errorf("Error forwarding to splunk: %s", buf.String())
	}
	return nil
}

func (sl *SplunkLogger) LogWithTime(t time.Time, msg string) error {
	sp := SplunkPayload{
		Time: t.Unix(),
		Host: sl.hostname,
		Event: SplunkEvent{
			Message: msg,
			Ident:   sl.source,
			Host:    sl.hostname,
		},
	}
	select {
	case sl.payloads <- &sp:
	default:
		return fmt.Errorf("Error queueing splunk payload, channel full")
	}
	return nil
}

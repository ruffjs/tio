package emqx

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/pkg/errors"
	"ruff.io/tio/pkg/log"
)

type clientPage struct {
	Data []ClientInfo   `json:"data"`
	Meta clientPageMeta `json:"meta"`
}

type clientPageMeta struct {
	Count int64 `json:"count"`
	Limit int   `json:"limit"`
	Page  int   `json:"page"`
}

func fetchClientPage(apiPrefix, apiToken string, page, limit uint) (clientPage, error) {
	api := fmt.Sprintf("%s/api/v5/clients?page=%d&limit=%d", apiPrefix, page, limit)
	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return clientPage{}, errors.Wrap(err, "new request")
	}
	req.Header.Set("Authorization", apiToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return clientPage{}, errors.Wrap(err, "fetch client page")
	}
	if resp.StatusCode != http.StatusOK {
		res, _ := io.ReadAll(resp.Body)
		log.Errorf("Fetch emqx client page http status=%d body=%q", resp.StatusCode, res)
		return clientPage{}, fmt.Errorf("fetch client page got http status %d", resp.StatusCode)
	}
	var c clientPage
	err = json.NewDecoder(resp.Body).Decode(&c)
	if err != nil {
		return clientPage{}, errors.Wrap(err, "decode response")
	}
	return c, nil
}

func fetchClient(apiPrefix, apiToken, thingId string) (ClientInfo, error) {
	api := apiPrefix + "/api/v5/clients/" + thingId
	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return ClientInfo{}, errors.Wrap(err, "new request")
	}
	req.Header.Set("Authorization", apiToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ClientInfo{}, errors.Wrap(err, "fetch client")
	}
	if resp.StatusCode == http.StatusNotFound {
		return ClientInfo{ClientId: thingId, Connected: false}, nil
	}
	if resp.StatusCode != http.StatusOK {
		res, _ := io.ReadAll(resp.Body)
		log.Errorf("Fetch emqx client info http status=%d body=%q", resp.StatusCode, res)
		return ClientInfo{}, fmt.Errorf("fetch client got http status %d", resp.StatusCode)
	}
	var c ClientInfo
	err = json.NewDecoder(resp.Body).Decode(&c)
	if err != nil {
		return ClientInfo{}, errors.Wrap(err, "decode response")
	}
	return c, nil
}

func closeClient(apiPrefix, apiToken, thingId string) error {
	api := apiPrefix + "/api/v5/clients/" + thingId
	req, err := http.NewRequest(http.MethodDelete, api, nil)
	if err != nil {
		return errors.Wrap(err, "new request")
	}
	req.Header.Set("Authorization", apiToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrapf(err, "close mqtt client %q", thingId)
	}
	if resp.StatusCode == http.StatusNotFound {
		log.Warnf("Close mqtt client %q, but got 404 status", thingId)
		return nil
	}
	if resp.StatusCode != http.StatusNoContent {
		res, _ := io.ReadAll(resp.Body)
		log.Errorf("Fetch mqtt client info http status=%d body=%q", resp.StatusCode, res)
		return fmt.Errorf("close client %q got http status %d", thingId, resp.StatusCode)
	} else {
		log.Infof("Closed mqtt client: clientId=%s", thingId)
	}
	return nil
}

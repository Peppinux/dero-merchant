package coingecko

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/peppinux/dero-merchant/stringutil"
)

const apiURL = "https://api.coingecko.com/api/v3"

func getEndpointBody(endpoint string, query string) (body []byte, err error) {
	url := stringutil.Build(apiURL, endpoint, query)

	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}
	resp, err := httpClient.Get(url)
	if err != nil {
		err = errors.Wrap(err, "cannot get endpoint")
		return
	}

	defer resp.Body.Close()

	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		err = errors.Wrap(err, "cannot read response")
	}
	return
}

// Ping checks CoinGecko API V3 server status
func Ping() (statusCode int) {
	url := stringutil.Build(apiURL, "/ping")

	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}
	resp, err := httpClient.Get(url)
	if err != nil {
		return
	}

	defer resp.Body.Close()

	statusCode = resp.StatusCode
	return
}

// SupportedVsCurrencies returns a list of currencies supported by the CoinGecko API V3
func SupportedVsCurrencies() (supportedVsCurrencies []string, err error) {
	var body []byte
	body, err = getEndpointBody("/simple/supported_vs_currencies", "")
	if err != nil {
		err = errors.Wrap(err, "cannot get endpoint body")
		return
	}

	err = json.Unmarshal(body, &supportedVsCurrencies)
	if err != nil {
		err = errors.Wrap(err, "cannot unmarshal response body")
	}
	return
}

// DeroPrice returns the price of Dero compared to a currency
func DeroPrice(vsCurrency string) (deroPrice float64, err error) {
	vsCurrency = strings.ToLower(vsCurrency)
	query := stringutil.Build("?ids=dero&vs_currencies=", vsCurrency)
	var body []byte
	body, err = getEndpointBody("/simple/price", query)
	if err != nil {
		err = errors.Wrap(err, "cannot get endpoint body")
		return
	}

	resp := make(map[string](map[string]float64))
	err = json.Unmarshal(body, &resp)
	if err != nil {
		err = errors.Wrap(err, "cannot unmarshal response body")
		return
	}

	deroPrice = resp["dero"][vsCurrency]
	return
}

package processor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"

	deroconfig "github.com/deroproject/derosuite/config"
	deroglobals "github.com/deroproject/derosuite/globals"

	"github.com/peppinux/dero-merchant/config"
	"github.com/peppinux/dero-merchant/stringutil"
)

// SetupDaemonConnection sets up network globals and checks if daemon is online and of the right type of network
func SetupDaemonConnection() error {
	daemonNetwork, err := getDaemonNetwork(config.DeroDaemonAddress)
	if err != nil {
		return errors.Wrap(err, "cannot get daemon network type")
	}

	deroglobals.Arguments = map[string]interface{}{}

	switch config.DeroNetwork {
	case "testnet":
		if daemonNetwork != "testnet" {
			return fmt.Errorf("DERO_NETWORK (testnet) and DERO_DAEMON_ADDRESS network (%s) not matching", daemonNetwork)
		}
		deroglobals.Arguments["--testnet"] = true
		deroglobals.Config = deroconfig.Testnet
	case "mainnet":
		if daemonNetwork != "mainnet" {
			return fmt.Errorf("DERO_NETWORK (mainnet) and DERO_DAEMON_ADDRESS network (%s) not matching", daemonNetwork)
		}
		deroglobals.Arguments["--testnet"] = false
		deroglobals.Config = deroconfig.Mainnet
	default:
		return errors.New("DERO_NETWORK env variable should be either \"testnet\" or \"mainnet\"")
	}

	return nil
}

func getDaemonNetwork(address string) (network string, err error) {
	params := map[string]string{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  "get_info",
	}
	reqBody, err := json.Marshal(params)
	if err != nil {
		err = errors.Wrap(err, "cannot marshal request body")
		return
	}

	url := stringutil.Build(address, "/json_rpc")
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		err = errors.Wrap(err, "error sending post request")
		return
	}

	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		err = errors.Wrap(err, "cannot read response body")
		return
	}

	respBodyJSON := struct {
		ID      string `json:"id"`
		JSONRPC string `json:"jsonrpc"`
		Result  map[string]interface{}
	}{}
	err = json.Unmarshal(respBody, &respBodyJSON)
	if err != nil {
		err = errors.Wrap(err, "cannot unmarshal response body")
		return
	}

	switch respBodyJSON.Result["testnet"] {
	case true:
		network = "testnet"
	case false:
		network = "mainnet"
	default:
		network = "unknown"
	}
	return
}

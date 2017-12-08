package tickets

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"
)

// defaultVerificationURL is The default URL that's used to verify the user's response to the challenge.
// @see https://developers.google.com/recaptcha/docs/verify#api-request
var VerificationURL = "https://www.google.com/recaptcha/api/siteverify"
var reCAPTCHASecret = ""
var reClient = &http.Client{Timeout: 20 * time.Second}

// Response is the JSON structure that is returned by the verification API after a challenge response is verified.
// @see https://developers.google.com/recaptcha/docs/verify#api-response
type Response struct {
	Success bool `json:"success"`

	// Timestamp of the challenge load (ISO format yyyy-MM-dd'T'HH:mm:ssZZ)
	Challenge string `json:"challenge_ts"`

	// The hostname of the site where the reCAPTCHA was solved
	Hostname string `json:"hostname"`

	// Optional list of error codes returned by the service
	ErrorCodes []string `json:"error-codes"`
}

// Verify the users's response to the reCAPTCHA challenge with the API server.
//
// The parameter response is obtained after the user successfully solves the challenge presented by the JS widget. The
// remoteip parameter is optional; just send it empty if you don't want to use it.
//
// CAPTCHAVerify function will return a boolean that will have the final result returned by the API as well as an optional list
// of errors. They might be useful for logging purposed but you don't have to show them to the user.
func CAPTCHAVerify(response string, remoteip string) (Response, error) {

	params := url.Values{}

	if reCAPTCHASecret == "" {
		reCAPTCHASecret = os.Getenv("ADVLIGHT_RECAPTCHA_SECRET")
	}

	params.Set("secret", reCAPTCHASecret)

	if len(response) > 0 {
		params.Set("response", response)
	}

	if net.ParseIP(remoteip) != nil {
		params.Set("remoteip", remoteip)
	}

	resp := Response{Success: false}

	r, err := reClient.PostForm(VerificationURL, params)

	if err != nil {
		return resp, err
	}

	defer r.Body.Close()

	if r.StatusCode != 200 {
		return resp, errors.New(r.Status)
	}

	json.NewDecoder(r.Body).Decode(&resp)
	if resp.ErrorCodes != nil && len(resp.ErrorCodes) > 0 {
		return resp, errors.New(resp.ErrorCodes[0])
	}
	return resp, nil

}

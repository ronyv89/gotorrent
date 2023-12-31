package ygg

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/ronyv89/gotorrent/core"
)

// loginURL is the url used to retrieve to authenticate user.
var loginURL = url.URL{
	Scheme: "https",
	Host:   baseURL,
	Path:   "user/login",
}

// authUser authenticates user and stores cookies so that authentication is memorized
func authUser(userID string, userPass string, client *http.Client) (*http.Client, error) {
	// Encode id and password as get parameters that will be passed to the request body
	formData := url.Values{
		"id":   {userID},
		"pass": {userPass},
	}

	// Create the POST request and put credentials in the body
	req, err := http.NewRequest("POST", loginURL.String(), strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("could not build POST request to login url: %v", err)
	}

	// Set proper headers.
	// Content-Type and Content-Length are not compulsory with Ygg but this is good practice.
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(formData.Encode())))
	req.Header.Set("User-Agent", core.UserAgent)

	// Launch request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST request to login url failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("authentication failed with status code %v", resp.StatusCode)
	}

	return client, nil
}

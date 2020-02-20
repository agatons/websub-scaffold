package main

import (
	"encoding/hex"
	"io/ioutil"
	"crypto/hmac"
	"crypto/sha256"
	"strings"
	"math/rand"
	"time"
	"fmt"
	"github.com/labstack/echo"
	"net/http"
)

type SubRequest struct {
	Callback	string	 `json:"hub.callback" form:"hub.callback"`
	Mode		string	 `json:"mode" form:"hub.mode"`
	Secret		string	 `json:"secret" form:"hub.secret"`
	Topic		string	 `json:"topic" form:"hub.topic"`
}


func randSeq(n int) string {
	chars := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")
	b := make([]rune, n)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func handleSubReq(c echo.Context, subscriber *SubRequest) error {
	client := http.Client{}

	// Get subscriber request values
	subRequest := SubRequest{}
	err := c.Bind(&subRequest)
	if err != nil {
		fmt.Println("Failed processing request: ", err)
		return c.String(http.StatusInternalServerError, "Failed processing request")
	}

	// Verify subscriber's request with a GET
	req, err := http.NewRequest("GET", subRequest.Callback, nil)
	if err != nil {
		fmt.Println("Failed creating GET request: ", err)
		return c.String(http.StatusInternalServerError, "Failed creating GET request")
	}

	// Create random challenge value
	challenge := randSeq(10)
	// Add query values
	q := req.URL.Query()
	q.Add("hub.mode", subRequest.Mode)
	q.Add("hub.topic", subRequest.Topic)
	q.Add("hub.challenge", challenge)
	req.URL.RawQuery = q.Encode()

	// Send GET request
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Failed sending GET request: ", err)
		return c.String(http.StatusInternalServerError, "Failed sending GET request")
	}

	// Handle response
	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Could not read GET response body: ", err)
		return c.String(http.StatusInternalServerError, "Failed reading GET response")
	}

	fmt.Println("Response from GET: ", resp)
	// Check if subscriber echos the right value
	if string(bodyBytes) != challenge {
		// Add subscription
		fmt.Println("Subscriber echoed the wrong value")
		return c.String(http.StatusInternalServerError, "Bad challenge echoed")
	}
	*subscriber = subRequest
	return c.String(http.StatusOK, "Subscription added")

}

func postSubscribers(subscriber SubRequest, message []byte) (string, error) {
	client := http.Client{}

	// HMAC signature
	hmac := hmac.New(sha256.New, []byte(subscriber.Secret))
	hmac.Write(message)
	shaValue := hex.EncodeToString(hmac.Sum(nil))

	// Create POST request
	req, err := http.NewRequest("POST", subscriber.Callback, strings.NewReader(string(message)))
	if err != nil {
		return "Failed creating POST request", err
	}
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Add("X-Hub-Signature", "sha256="+shaValue)

	_, err = client.Do(req)
	if err != nil {
		return "Error sending POST request", err
	}
	return "Posted to subscribers", nil


}

func main() {
	// Random seed used for the "hub.challenge"
 	rand.Seed(time.Now().UnixNano())

	// Should be subscribers but for this example I assume one subscriber is fine
	var subscriber SubRequest
	// Echo instance
	e := echo.New()

	// Setup publish endpoint
	e.POST("/publish", func(c echo.Context) error {
		// Call with curl command:
		//	  curl -H "Content-Type: application/json" -X POST -d '{<insert json>}' http://192.168.99.100:8080/publish

		data, err := ioutil.ReadAll(c.Request().Body)
		if err != nil {
			return c.String(http.StatusInternalServerError, "Could not read JSON")
		}

		// Check for subscriber
		if subscriber == (SubRequest{}) {
			return c.String(http.StatusInternalServerError, "No subscriber")
		}

		msg, err := postSubscribers(subscriber, data)
		if err != nil {
			return c.String(http.StatusInternalServerError, msg)
		}

		return c.String(http.StatusOK, msg)
	})

	// Handle subscription requests
	e.POST("/", func(c echo.Context) error {
		return handleSubReq(c, &subscriber)
	})

	// Start server
	e.Logger.Fatal(e.Start(":8080"))
}
// Copyright 2014 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tests

import (
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/github"
)

const MsgEnvarMissing = "Skipping test because the required environment variable (%v) is not present."
const EnvarKeyGitHubUsername = "GITHUB_USERNAME"
const EnvarKeyGitHubPassword = "GITHUB_PASSWORD"
const EnvarKeyClientID = "GITHUB_CLIENT_ID"
const EnvarKeyClientSecret = "GITHUB_CLIENT_SECRET"
const InvalidTokenValue = "iamnotacroken"

// TestAuthorizationsBasicOperations tests the basic CRUD operations of the API (mostly for
// the Personal Access Token scenario).
func TestAuthorizationsBasicOperations(t *testing.T) {

	client := getUserPassClient(t)

	auths, resp, err := client.Authorizations.List()
	failOnError(t, err)
	failIfNotStatusCode(t, resp, 200)

	initialAuthCount := len(auths)

	authReq := generatePersonalAuthTokenRequest()

	createdAuth, resp, err := client.Authorizations.Create(authReq)
	failOnError(t, err)
	failIfNotStatusCode(t, resp, 201)

	if *authReq.Note != *createdAuth.Note {
		t.Fatal("Returned Authorization does not match the requested Authorization.")
	}

	auths, resp, err = client.Authorizations.List()
	failOnError(t, err)
	failIfNotStatusCode(t, resp, 200)

	if len(auths) != initialAuthCount+1 {
		t.Fatalf("The number of Authorizations should have increased. Expected [%v], was [%v]", (initialAuthCount + 1), len(auths))
	}

	// Test updating the authorization
	authUpdate := new(github.AuthorizationUpdate)
	authUpdate.ID = createdAuth.ID
	authUpdate.Note = github.String("Updated note: " + randString())

	updatedAuth, resp, err := client.Authorizations.Edit(authUpdate)
	failOnError(t, err)
	failIfNotStatusCode(t, resp, 200)

	if *updatedAuth.Note != *authUpdate.Note {
		t.Fatal("The returned Authorization does not match the requested updated value.")
	}

	// Verify that the Get operation also reflects the update
	retrievedAuth, resp, err := client.Authorizations.Get(*createdAuth.ID)
	failOnError(t, err)
	failIfNotStatusCode(t, resp, 200)

	if *retrievedAuth.Note != *updatedAuth.Note {
		t.Fatal("The retrieved Authorization does not match the expected (updated) value.")
	}

	// Now, let's delete...
	resp, err = client.Authorizations.Delete(*createdAuth.ID)
	failOnError(t, err)
	failIfNotStatusCode(t, resp, 204)

	// Verify that we can no longer retrieve the auth
	retrievedAuth, resp, err = client.Authorizations.Get(*createdAuth.ID)
	if err == nil {
		t.Fatal("Should have failed due to 404")
	}
	failIfNotStatusCode(t, resp, 404)

	// Verify that our count reset back to the initial value
	auths, resp, err = client.Authorizations.List()
	failOnError(t, err)
	failIfNotStatusCode(t, resp, 200)

	if len(auths) != initialAuthCount {
		t.Fatal("The number of Authorizations should match the initial count Expected [%v], got [%v]", (initialAuthCount), len(auths))
	}

}

// TestAuthorizationsAppOperations tests the application/token related operations, such
// as creating, testing, resetting and revoking application OAuth tokens.
func TestAuthorizationsAppOperations(t *testing.T) {

	userAuthenticatedClient := getUserPassClient(t)

	appAuthenticatedClient := getOAuthAppClient(t)

	// We know these vars are set because getOAuthAppClient would have
	// skipped the test by now
	clientID := os.Getenv(EnvarKeyClientID)
	clientSecret := os.Getenv(EnvarKeyClientSecret)

	authRequest := generateAppAuthTokenRequest(clientID, clientSecret)

	createdAuth, resp, err := userAuthenticatedClient.Authorizations.GetOrCreateForApp(authRequest)
	failOnError(t, err)
	failIfNotStatusCode(t, resp, 201)

	// Quick sanity check:
	if *createdAuth.Note != *authRequest.Note {
		t.Fatal("The returned auth does not match expected value.")
	}

	// Let's try the same request again, this time it should return the same
	// auth instead of creating a new one
	secondAuth, resp, err := userAuthenticatedClient.Authorizations.GetOrCreateForApp(authRequest)
	failOnError(t, err)
	failIfNotStatusCode(t, resp, 200)

	// Verify that the IDs are the same
	if *createdAuth.ID != *secondAuth.ID {
		t.Fatalf("The ID of the second returned auth should be the same as the first. Expected [%v], got [%v]", createdAuth.ID, secondAuth.ID)
	}

	// Verify the token
	appAuth, resp, err := appAuthenticatedClient.Authorizations.CheckAppAuthorization(clientID, *createdAuth.Token)
	failOnError(t, err)
	failIfNotStatusCode(t, resp, 200)

	// Quick sanity check
	if *appAuth.ID != *createdAuth.ID || *appAuth.Token != *createdAuth.Token {
		t.Fatal("The returned auth/token does not match.")
	}

	// Let's verify that we get a 404 for a non-existent token
	_, resp, err = appAuthenticatedClient.Authorizations.CheckAppAuthorization(clientID, InvalidTokenValue)
	if err == nil {
		t.Fatal("An error should have been returned because of the invalid token.")
	}
	failIfNotStatusCode(t, resp, 404)

	// Let's reset the token
	resetAuth, resp, err := appAuthenticatedClient.Authorizations.ResetAppAuthorization(clientID, *createdAuth.Token)
	failOnError(t, err)
	failIfNotStatusCode(t, resp, 200)

	// Let's verify that we get a 404 for a non-existent token
	_, resp, err = appAuthenticatedClient.Authorizations.ResetAppAuthorization(clientID, InvalidTokenValue)
	if err == nil {
		t.Fatal("An error should have been returned because of the invalid token.")
	}
	failIfNotStatusCode(t, resp, 404)

	// Verify that the token has changed
	if resetAuth.Token == createdAuth.Token {
		t.Fatal("The reset token should be different from the original.")
	}

	// Verify that we do have a token value
	if *resetAuth.Token == "" {
		t.Fatal("A token value should have been returned.")
	}

	// Verify that the original token is now invalid
	_, resp, err = appAuthenticatedClient.Authorizations.CheckAppAuthorization(clientID, *createdAuth.Token)
	if err == nil {
		t.Fatal("The original token should be invalid.")
	}
	failIfNotStatusCode(t, resp, 404)

	// Check that the reset token is valid
	_, resp, err = appAuthenticatedClient.Authorizations.CheckAppAuthorization(clientID, *resetAuth.Token)
	failOnError(t, err)
	failIfNotStatusCode(t, resp, 200)

	// Let's revoke the token
	resp, err = appAuthenticatedClient.Authorizations.RevokeAppAuthorization(clientID, *resetAuth.Token)
	failOnError(t, err)
	failIfNotStatusCode(t, resp, 204)

	// Sleep for two seconds... I've seen cases where the revocation appears not
	// to have take place immediately.
	time.Sleep(time.Second * 2)

	// Now, the reset token should also be invalid
	_, resp, err = appAuthenticatedClient.Authorizations.CheckAppAuthorization(clientID, *resetAuth.Token)
	if err == nil {
		t.Fatal("The reset token should be invalid.")
	}
	failIfNotStatusCode(t, resp, 404)
}

// generatePersonalAuthTokenRequest is a helper function that generates an
// AuthorizationRequest for a Personal Access Token (no client id).
func generatePersonalAuthTokenRequest() *github.AuthorizationRequest {

	rand := randString()
	auth := github.AuthorizationRequest{
		Note:        github.String("Personal token: Note generated by test: " + rand),
		Scopes:      []github.Scope{github.ScopePublicRepo},
		Fingerprint: github.String("Personal token: Fingerprint generated by test: " + rand),
	}

	return &auth
}

// generatePersonalAuthTokenRequest is a helper function that generates an
// AuthorizationRequest for an OAuth application Token (uses client id).
func generateAppAuthTokenRequest(clientID string, clientSecret string) *github.AuthorizationRequest {

	rand := randString()
	auth := github.AuthorizationRequest{
		Note:         github.String("App token: Note generated by test: " + rand),
		Scopes:       []github.Scope{github.ScopePublicRepo},
		Fingerprint:  github.String("App token: Fingerprint generated by test: " + rand),
		ClientID:     github.String(clientID),
		ClientSecret: github.String(clientSecret),
	}

	return &auth
}

// randString returns a (kinda) random string for uniqueness purposes.
func randString() string {
	return strconv.FormatInt(rand.NewSource(time.Now().UnixNano()).Int63(), 10)
}

// failOnError invokes t.Fatal() if err is present.
func failOnError(t *testing.T, err error) {

	if err != nil {
		t.Fatal(err)
	}
}

// failIfNotStatusCode invokes t.Fatal() if the response's status code doesn't match the expected code.
func failIfNotStatusCode(t *testing.T, resp *github.Response, expectedCode int) {

	if resp.StatusCode != expectedCode {
		t.Fatalf("Expected HTTP status code [%v] but received [%v]", expectedCode, resp.StatusCode)
	}

}

// getUserPassClient returns a GitHub client for authorization testing. The client
// uses BasicAuth via GH username and password passed in environment variables
// (and will skip the calling test if those vars are not present).
func getUserPassClient(t *testing.T) *github.Client {
	username, isPresent := os.LookupEnv(EnvarKeyGitHubUsername)
	if !isPresent {
		t.Skipf(MsgEnvarMissing, EnvarKeyGitHubUsername)
	}

	password, isPresent := os.LookupEnv(EnvarKeyGitHubPassword)
	if !isPresent {
		t.Skipf(MsgEnvarMissing, EnvarKeyGitHubPassword)
	}

	tp := github.BasicAuthTransport{
		Username: strings.TrimSpace(username),
		Password: strings.TrimSpace(password),
	}

	return github.NewClient(tp.Client())
}

// getOAuthAppClient returns a GitHub client for authorization testing. The client
// uses BasicAuth, but instead of username and password, it uses the client id
// and client secret passed in via environment variables
// (and will skip the calling test if those vars are not present). Certain API operations (check
// an authorization; reset an authorization; revoke an authorization for an app)
// require this authentication mechanism.
//
// See GitHub API docs: https://developer.com/v3/oauth_authorizations/#check-an-authorization
func getOAuthAppClient(t *testing.T) *github.Client {

	username, isPresent := os.LookupEnv(EnvarKeyClientID)
	if !isPresent {
		t.Skipf(MsgEnvarMissing, EnvarKeyClientID)
	}

	password, isPresent := os.LookupEnv(EnvarKeyClientSecret)
	if !isPresent {
		t.Skipf(MsgEnvarMissing, EnvarKeyClientSecret)
	}

	tp := github.BasicAuthTransport{
		Username: strings.TrimSpace(username),
		Password: strings.TrimSpace(password),
	}

	return github.NewClient(tp.Client())
}

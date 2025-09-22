package auth

// MockTokenParams implements the TokenParams interface for use
// in tests.
type MockTokenParams struct {
	ServerURL string
}

func (mtp MockTokenParams) GetClientID() string {
	return "test_client_id"
}

func (mtp MockTokenParams) getClientSecret() string {
	return "test_client_secret"
}

func (mtp MockTokenParams) GetScope() string {
	return "*"
}

func (mtp MockTokenParams) GetAuthURL() string {
	return mtp.ServerURL
}

func (mtp MockTokenParams) GetApiURL() string {
	return mtp.ServerURL
}

func (mtp MockTokenParams) IsValid() error {
	return nil
}

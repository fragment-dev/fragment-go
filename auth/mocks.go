package auth

// MockTokenParams implements the TokenParams interface for use
// in tests.
type MockTokenParams struct {
	ServerUrl string
}

func (mtp MockTokenParams) GetClientId() string {
	return "test_client_id"
}

func (mtp MockTokenParams) GetClientSecret() string {
	return "test_client_secret"
}

func (mtp MockTokenParams) GetScope() string {
	return "*"
}

func (mtp MockTokenParams) GetAuthUrl() string {
	return mtp.ServerUrl
}

func (mtp MockTokenParams) GetApiUrl() string {
	return mtp.ServerUrl
}

func (mtp MockTokenParams) IsValid() error {
	return nil
}

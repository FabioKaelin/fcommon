package values

type (
	Values struct {
		GinMode              string
		JsonLogs             bool
		OAuthFrontendServer  string
		OAuthBackendInternal string
		ImageServiceInternal string
		NotificationID       string
		DatabaseValues       DatabaseValues
		FVersion             string
	}

	DatabaseValues struct {
		DatabaseUser     string
		DatabasePassword string
		DatabaseHost     string
		DatabasePort     string
		DatabaseName     string
	}
)

var V = Values{}

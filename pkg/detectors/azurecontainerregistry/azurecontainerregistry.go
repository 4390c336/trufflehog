package azurecontainerregistry

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"encoding/base64"

	"github.com/trufflesecurity/trufflehog/v3/pkg/common"
	"github.com/trufflesecurity/trufflehog/v3/pkg/detectors"
	"github.com/trufflesecurity/trufflehog/v3/pkg/pb/detectorspb"
)

type Scanner struct {
	client *http.Client
}

// Ensure the Scanner satisfies the interface at compile time.
var _ detectors.Detector = (*Scanner)(nil)

var (
	defaultClient = common.SaneHttpClient()
	// Make sure that your group is surrounded in boundary characters such as below to reduce false positives.
    url = regexp.MustCompile(`([a-zA-Z0-9-]{1,100})\.azurecr\.io`)
    password = regexp.MustCompile(`[A-Za-z0-9+/=]{52}`)
)

// Keywords are used for efficiently pre-filtering chunks.
// Use identifiers in the secret preferably, or the provider name.
func (s Scanner) Keywords() []string {
	return []string{".azurecr.io"}
}

// FromData will find and optionally verify Azurecontainerregistry secrets in a given set of bytes.
func (s Scanner) FromData(ctx context.Context, verify bool, data []byte) (results []detectors.Result, err error) {
	dataStr := string(data)

	urlMatches := url.FindAllStringSubmatch(dataStr, -1)
	passwordMatches := password.FindAllStringSubmatch(dataStr, -1)

	for _, urlMatch := range urlMatches {
		for _, passwordMatch := range passwordMatches {

			endpoint := urlMatch[0]
			username := urlMatch[1]
			password := passwordMatch[0]

			s1 := detectors.Result{
				DetectorType: detectorspb.DetectorType_AzureContainerRegistry,
				Raw:          []byte(endpoint),
				Redacted:     endpoint,
				RawV2:        []byte(endpoint + password),
			}

			if verify {
				client := s.client
				if client == nil {
					client = defaultClient
				}

				auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))
				url := fmt.Sprintf("https://%s/v2/", endpoint)
				req, err := http.NewRequest("GET", url, nil)
				if err != nil {
					continue
				}

				req.Header.Set("Authorization", fmt.Sprintf("Basic %s", auth))
				resp, err := client.Do(req)
				if err != nil {
					continue
				}
				defer resp.Body.Close()

				if resp.StatusCode == http.StatusOK {
					s1.Verified = true
				}
			}

			if !s1.Verified && detectors.IsKnownFalsePositive(password, detectors.DefaultFalsePositives, true) {
				continue
			}

			results = append(results, s1)
			if s1.Verified {
				break
			}
		}
	}

	return results, nil
}

func (s Scanner) Type() detectorspb.DetectorType {
	return detectorspb.DetectorType_AzureContainerRegistry
}
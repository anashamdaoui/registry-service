package middleware

import (
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// IsValidURL checks if the url string is valid
/*
1. URL Parsing: The net/url package is used to parse the address string. This helps ensure that the string is a properly formatted URL and can detect obvious structural errors.
2. Scheme Validation: After parsing, we check that the scheme is either http or https. This prevents other schemes (e.g., file, ftp, etc.) from being accepted.
3. Host Validation: We ensure that the host part of the URL is not empty, preventing URLs that don't point to a valid domain.
4. Regular Expression: The regex pattern is used to match common URL components while filtering out potentially malicious or malformed data. The pattern matches:
- The scheme (either http or https).
- A valid URL path and query string, allowing for most typical characters found in URLs.
*/
func IsValidURL(inURL string) bool {
	// Check if the address can be parsed as a URL
	parsedURL, err := url.Parse(inURL)
	if err != nil {
		return false
	}

	// Ensure the scheme is either http or https
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return false
	}

	// Check that the host is not empty
	if parsedURL.Host == "" {
		return false
	}

	// Check for invalid characters using a regular expression
	// This regex checks for a simple URL pattern: scheme://host/path
	// Adjust the pattern according to your specific security requirements
	var urlRegex = regexp.MustCompile(`^(http|https):\/\/[a-zA-Z0-9-._~:\/?#\[\]@!$&'()*+,;=%]+$`)
	return urlRegex.MatchString(inURL)
}

func GetHostAndPortFromURL(rawURL string) (string, int32, error) {
	if !strings.Contains(rawURL, "://") {
		// Add a default scheme
		rawURL = "http://" + rawURL
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", -1, err
	}

	host, portstr, err := net.SplitHostPort(parsedURL.Host)
	if err != nil {
		return "", -1, err
	}

	port, _ := strconv.Atoi(portstr)

	return host, int32(port), nil
}

func GetURLFromHostPort(host string, port int32) string {

	portStr := strconv.Itoa(int(port))
	url := "http://" + net.JoinHostPort(host, portStr)

	return url
}

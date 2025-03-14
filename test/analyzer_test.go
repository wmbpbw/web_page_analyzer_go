// test/analyzer_test.go

package test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/html"
)

// parseHTML is a helper function moved from the original test file
func parseHTML(htmlStr string) (*html.Node, error) {
	return html.Parse(strings.NewReader(htmlStr))
}

// detectHTMLVersion determines the HTML version from the document
// This is a copy of the function from internal/analyzer/analyzer.go
func detectHTMLVersion(n *html.Node) string {
	// Look for doctype declaration
	if n.Type == html.DoctypeNode {
		// HTML5
		if n.Attr == nil || len(n.Attr) == 0 {
			return "HTML5"
		}

		// Check for HTML 4.01
		for _, attr := range n.Attr {
			if strings.Contains(attr.Val, "HTML 4.01") {
				return "HTML 4.01"
			} else if strings.Contains(attr.Val, "XHTML 1.0") {
				return "XHTML 1.0"
			} else if strings.Contains(attr.Val, "XHTML 1.1") {
				return "XHTML 1.1"
			}
		}
	}

	// Recursively check children for doctype
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if version := detectHTMLVersion(c); version != "" {
			return version
		}
	}

	// Default to HTML5 if we can't determine version
	return "HTML5 (assumed)"
}

// detectLoginForm checks if a form is likely a login form
// This is a copy of the function from internal/analyzer/analyzer.go
func detectLoginForm(n *html.Node) bool {
	// Check for password input
	hasPasswordInput := false
	hasUsernameInput := false

	// Check for common login-related form attributes
	for _, attr := range n.Attr {
		if attr.Key == "id" || attr.Key == "name" || attr.Key == "class" {
			val := strings.ToLower(attr.Val)
			if strings.Contains(val, "login") || strings.Contains(val, "signin") || strings.Contains(val, "log-in") || strings.Contains(val, "sign-in") {
				return true
			}
		}
	}

	// Recursively search for password and username/email inputs
	var searchInputs func(*html.Node)
	searchInputs = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "input" {
			inputType := ""
			inputName := ""

			for _, attr := range node.Attr {
				if attr.Key == "type" {
					inputType = attr.Val
				} else if attr.Key == "name" || attr.Key == "id" {
					inputName = strings.ToLower(attr.Val)
				}
			}

			// Check for password input
			if inputType == "password" {
				hasPasswordInput = true
			}

			// Check for username/email input
			if (inputType == "text" || inputType == "email") &&
				(strings.Contains(inputName, "user") ||
					strings.Contains(inputName, "email") ||
					strings.Contains(inputName, "login") ||
					strings.Contains(inputName, "name")) {
				hasUsernameInput = true
			}
		}

		// Check children
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			searchInputs(c)
		}
	}

	searchInputs(n)

	// If we have both password and username/email inputs, it's likely a login form
	return hasPasswordInput && hasUsernameInput
}

// TestDetectHTMLVersion tests the detectHTMLVersion function
func TestDetectHTMLVersion(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{
			name:     "HTML5",
			html:     `<!DOCTYPE html><html><head></head><body></body></html>`,
			expected: "HTML5",
		},
		{
			name:     "HTML 4.01",
			html:     `<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01//EN" "http://www.w3.org/TR/html4/strict.dtd"><html><head></head><body></body></html>`,
			expected: "HTML 4.01",
		},
		{
			name:     "XHTML 1.0",
			html:     `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Strict//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-strict.dtd"><html><head></head><body></body></html>`,
			expected: "XHTML 1.0",
		},
		{
			name:     "No DOCTYPE",
			html:     `<html><head></head><body></body></html>`,
			expected: "HTML5 (assumed)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := parseHTML(tt.html)
			assert.NoError(t, err)

			version := detectHTMLVersion(doc)
			assert.Equal(t, tt.expected, version)
		})
	}
}

// TestDetectLoginForm tests the detectLoginForm function
func TestDetectLoginForm(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected bool
	}{
		{
			name: "Login Form",
			html: `<form id="login">
				<input type="text" name="username">
				<input type="password" name="password">
				<button type="submit">Login</button>
			</form>`,
			expected: true,
		},
		{
			name: "Login Form with class",
			html: `<form class="login-form">
				<input type="email" name="email">
				<input type="password" name="password">
				<button type="submit">Sign In</button>
			</form>`,
			expected: true,
		},
		{
			name: "Not a Login Form",
			html: `<form id="contact">
				<input type="text" name="name">
				<input type="email" name="email">
				<textarea name="message"></textarea>
				<button type="submit">Send</button>
			</form>`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := parseHTML(tt.html)
			assert.NoError(t, err)

			isLoginForm := detectLoginForm(doc)
			assert.Equal(t, tt.expected, isLoginForm)
		})
	}
}

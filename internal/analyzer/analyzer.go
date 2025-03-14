package analyzer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
	_ "golang.org/x/sync/errgroup"
	"log/slog"
	"webPageAnalyzerGO/internal/config"
	"webPageAnalyzerGO/internal/models"
)

// Analyzer handles URL analysis
type Analyzer struct {
	client *http.Client
	config config.AnalyzerConfig
	logger *slog.Logger
}

// New creates a new Analyzer
func New(cfg config.AnalyzerConfig, logger *slog.Logger) *Analyzer {
	return &Analyzer{
		client: &http.Client{
			Timeout: cfg.RequestTimeout,
		},
		config: cfg,
		logger: logger,
	}
}

// AnalyzeURL analyzes a webpage and returns the analysis results
func (a *Analyzer) AnalyzeURL(ctx context.Context, urlStr string) (*models.AnalysisResult, error) {
	// Parse URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Ensure scheme is set
	if parsedURL.Scheme == "" {
		parsedURL.Scheme = "https"
		urlStr = parsedURL.String()
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set User-Agent
	req.Header.Set("User-Agent", a.config.UserAgent)

	// Send request
	a.logger.Info("Sending request", "url", urlStr)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	// Read body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse HTML
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Analyze the document
	analysis := &models.AnalysisResult{
		URL:       urlStr,
		CreatedAt: time.Now(),
	}

	// Process the document
	a.analyzeDocument(ctx, doc, parsedURL, analysis)

	return analysis, nil
}

// analyzeDocument processes the parsed HTML document and populates the analysis
func (a *Analyzer) analyzeDocument(ctx context.Context, n *html.Node, baseURL *url.URL, analysis *models.AnalysisResult) {
	// Detect HTML version from doctype
	analysis.HTMLVersion = a.detectHTMLVersion(n)

	// Create maps to track links and avoid duplicates
	internalLinks := make(map[string]bool)
	externalLinks := make(map[string]bool)
	internalInaccessible := 0
	externalInaccessible := 0

	// Use WaitGroup to wait for all link checks
	var wg sync.WaitGroup

	// Process the entire document
	var processNode func(*html.Node)
	processNode = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "title":
				if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
					analysis.Title = n.FirstChild.Data
				}
			case "h1", "h2", "h3", "h4", "h5", "h6":
				// Increment heading count
				switch n.Data {
				case "h1":
					analysis.Headings.H1++
				case "h2":
					analysis.Headings.H2++
				case "h3":
					analysis.Headings.H3++
				case "h4":
					analysis.Headings.H4++
				case "h5":
					analysis.Headings.H5++
				case "h6":
					analysis.Headings.H6++
				}
			case "a":
				// Process links
				for _, attr := range n.Attr {
					if attr.Key == "href" {
						linkURL := attr.Val
						if linkURL == "" || strings.HasPrefix(linkURL, "#") {
							continue // Skip empty links and anchors
						}

						// Parse the link URL relative to the base URL
						parsedLink, err := url.Parse(linkURL)
						if err != nil {
							continue // Skip invalid URLs
						}

						resolvedURL := baseURL.ResolveReference(parsedLink)

						// Determine if internal or external
						if resolvedURL.Host == baseURL.Host {
							if _, exists := internalLinks[resolvedURL.String()]; !exists {
								internalLinks[resolvedURL.String()] = true

								// Check link accessibility in a goroutine
								wg.Add(1)
								go func(link string) {
									defer wg.Done()
									if !a.isLinkAccessible(ctx, link) {
										internalInaccessible++
									}
								}(resolvedURL.String())
							}
						} else {
							if _, exists := externalLinks[resolvedURL.String()]; !exists {
								externalLinks[resolvedURL.String()] = true

								// Check link accessibility in a goroutine
								wg.Add(1)
								go func(link string) {
									defer wg.Done()
									if !a.isLinkAccessible(ctx, link) {
										externalInaccessible++
									}
								}(resolvedURL.String())
							}
						}
					}
				}
			case "form":
				// Check for login form
				if !analysis.HasLoginForm {
					analysis.HasLoginForm = a.detectLoginForm(n)
				}
			}
		}

		// Recursively process child nodes
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			processNode(c)
		}
	}

	// Process the document
	processNode(n)

	// Wait for all link checks to complete
	wg.Wait()

	// Set link counts in the analysis result
	analysis.InternalLinks = models.LinkStatus{
		Count:        len(internalLinks),
		Inaccessible: internalInaccessible,
	}
	analysis.ExternalLinks = models.LinkStatus{
		Count:        len(externalLinks),
		Inaccessible: externalInaccessible,
	}
}

// detectHTMLVersion determines the HTML version from the document
func (a *Analyzer) detectHTMLVersion(n *html.Node) string {
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
		if version := a.detectHTMLVersion(c); version != "" {
			return version
		}
	}

	// Default to HTML5 if we can't determine version
	return "HTML5 (assumed)"
}

// detectLoginForm checks if a form is likely a login form
func (a *Analyzer) detectLoginForm(n *html.Node) bool {
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

// isLinkAccessible checks if a link is accessible by sending a HEAD request
func (a *Analyzer) isLinkAccessible(ctx context.Context, link string) bool {
	// Create a client with a short timeout for link checking
	client := &http.Client{
		Timeout: 3 * time.Second, // Short timeout for link checking
	}

	// Use HEAD request to minimize bandwidth usage
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, link, nil)
	if err != nil {
		return false
	}

	req.Header.Set("User-Agent", a.config.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Consider 2xx and 3xx status codes as accessible
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}

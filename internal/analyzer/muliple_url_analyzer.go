package analyzer

import (
	"context"
	"fmt"
	"golang.org/x/net/html"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"golang.org/x/time/rate"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"webPageAnalyzerGO/internal/config"
	"webPageAnalyzerGO/internal/models"
)

// Analyzer handles URL analysis
type MultipleUrlAnalyzer struct {
	client     *http.Client
	config     config.AnalyzerConfig
	logger     *slog.Logger
	limiter    *rate.Limiter
	maxWorkers int64
	semaphore  *semaphore.Weighted
}

// AnalyzerOptions contains configurable options for the analyzer
type AnalyzerOptions struct {
	MaxConcurrentRequests int64
	RequestsPerSecond     rate.Limit
	MaxMemoryMB           int64
}

// DefaultAnalyzerOptions returns sensible default options
func DefaultAnalyzerOptions() AnalyzerOptions {
	return AnalyzerOptions{
		MaxConcurrentRequests: 100,
		RequestsPerSecond:     10,
		MaxMemoryMB:           1024, // 1GB max memory usage
	}
}

// New creates a new Analyzer
func NewMultipleUrlAnalyzer(cfg config.AnalyzerConfig, logger *slog.Logger, opts AnalyzerOptions) *MultipleUrlAnalyzer {
	return &MultipleUrlAnalyzer{
		client: &http.Client{
			Timeout: cfg.RequestTimeout,
		},
		config:     cfg,
		logger:     logger,
		limiter:    rate.NewLimiter(opts.RequestsPerSecond, 1), // Allow bursts of 1
		maxWorkers: opts.MaxConcurrentRequests,
		semaphore:  semaphore.NewWeighted(opts.MaxMemoryMB * 1024 * 1024), // Convert MB to bytes
	}
}

// AnalyzeURLs analyzes multiple webpages concurrently and returns the analysis results
func (a *MultipleUrlAnalyzer) AnalyzeURLs(ctx context.Context, urls []string) ([]*models.AnalysisResult, error) {
	// Create a channel for results with sufficient buffer to avoid blocking
	resultChan := make(chan *models.AnalysisResult, len(urls))
	errorChan := make(chan error, len(urls))

	// Use errgroup to manage workers and propagate errors
	g, ctx := errgroup.WithContext(ctx)

	// Create a worker pool
	var wg sync.WaitGroup
	workerCh := make(chan string, a.maxWorkers)

	// Start worker goroutines
	for i := 0; i < int(a.maxWorkers); i++ {
		g.Go(func() error {
			for urlStr := range workerCh {
				// Wait for rate limiter
				if err := a.limiter.Wait(ctx); err != nil {
					errorChan <- fmt.Errorf("rate limiter error: %w", err)
					continue
				}

				// Analyze URL
				result, err := a.analyzeURLMulti(ctx, urlStr)
				if err != nil {
					errorChan <- fmt.Errorf("error analyzing %s: %w", urlStr, err)
				} else if result != nil {
					resultChan <- result
				}

				wg.Done()
			}
			return nil
		})
	}

	// Queue URLs to be processed
	wg.Add(len(urls))
	go func() {
		for _, urlStr := range urls {
			select {
			case <-ctx.Done():
				return
			case workerCh <- urlStr:
				// URL queued successfully
			}
		}
	}()

	// Close channels when all work is done
	go func() {
		wg.Wait()
		close(workerCh)
		close(resultChan)
		close(errorChan)
	}()

	// Collect results and errors
	var results []*models.AnalysisResult
	var errs []error

	// Process results channel
	for result := range resultChan {
		results = append(results, result)
	}

	// Process errors channel
	for err := range errorChan {
		errs = append(errs, err)
	}

	// Wait for all workers to complete
	if err := g.Wait(); err != nil {
		return results, fmt.Errorf("worker error: %w", err)
	}

	// If there were errors, combine them into a single error
	if len(errs) > 0 {
		errorMsg := fmt.Sprintf("%d errors occurred during analysis", len(errs))
		// Include up to 5 error messages in the response
		if len(errs) > 0 {
			errorMsg += ":"
			for i := 0; i < min(5, len(errs)); i++ {
				errorMsg += fmt.Sprintf("\n- %s", errs[i].Error())
			}
			if len(errs) > 5 {
				errorMsg += fmt.Sprintf("\n- ... and %d more errors", len(errs)-5)
			}
		}
		return results, fmt.Errorf(errorMsg)
	}

	return results, nil
}

// analyzeURL analyzes a single webpage and returns the analysis result
func (a *MultipleUrlAnalyzer) analyzeURLMulti(ctx context.Context, urlStr string) (*models.AnalysisResult, error) {
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
	a.logger.Debug("Sending request", "url", urlStr)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	// Estimate memory needed for this page
	contentLength := resp.ContentLength
	if contentLength <= 0 {
		contentLength = 1024 * 1024 // Assume 1MB if unknown
	}
	// Reserve memory for page processing (multiply by 5 to account for parsing overhead)
	estimatedMemory := contentLength * 5

	// Acquire semaphore (memory resources)
	if err := a.semaphore.Acquire(ctx, estimatedMemory); err != nil {
		return nil, fmt.Errorf("resource acquisition failed: %w", err)
	}
	defer a.semaphore.Release(estimatedMemory)

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
	a.analyzeDocumentMulti(ctx, doc, parsedURL, analysis)

	return analysis, nil
}

// analyzeDocument processes the parsed HTML document and populates the analysis
func (a *MultipleUrlAnalyzer) analyzeDocumentMulti(ctx context.Context, n *html.Node, baseURL *url.URL, analysis *models.AnalysisResult) {
	// Detect HTML version from doctype
	analysis.HTMLVersion = a.detectHTMLVersionMulti(n)

	// Create maps to track links and avoid duplicates
	internalLinks := make(map[string]bool)
	externalLinks := make(map[string]bool)
	internalInaccessible := 0
	externalInaccessible := 0

	// Use a limited worker pool for link checking to avoid creating too many goroutines
	linkCh := make(chan string, 100) // Buffer channel to avoid blocking
	var linkWg sync.WaitGroup

	// Start a limited number of link checker goroutines
	maxLinkCheckers := 20
	for i := 0; i < maxLinkCheckers; i++ {
		go func() {
			for link := range linkCh {
				isInternal := strings.HasPrefix(link, baseURL.String())
				if !a.isLinkAccessibleMulti(ctx, link) {
					if isInternal {
						internalInaccessible++
					} else {
						externalInaccessible++
					}
				}
				linkWg.Done()
			}
		}()
	}

	// Process the entire document
	var processNode func(*html.Node)
	processNode = func(n *html.Node) {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return
		default:
			// Continue processing
		}

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

								// Queue link for accessibility check
								linkWg.Add(1)
								select {
								case linkCh <- resolvedURL.String():
									// Successfully queued
								default:
									// Channel full, consider link as accessible to avoid blocking
									linkWg.Done()
								}
							}
						} else {
							if _, exists := externalLinks[resolvedURL.String()]; !exists {
								externalLinks[resolvedURL.String()] = true

								// Queue link for accessibility check
								linkWg.Add(1)
								select {
								case linkCh <- resolvedURL.String():
									// Successfully queued
								default:
									// Channel full, consider link as accessible to avoid blocking
									linkWg.Done()
								}
							}
						}
					}
				}
			case "form":
				// Check for login form
				if !analysis.HasLoginForm {
					analysis.HasLoginForm = a.detectLoginFormMulti(n)
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

	// Signal no more links and wait for all link checks to complete
	linkWg.Wait()
	close(linkCh)

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

// detectHTMLVersion and detectLoginForm methods remain unchanged
func (a *MultipleUrlAnalyzer) detectHTMLVersionMulti(n *html.Node) string {
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
		if version := a.detectHTMLVersionMulti(c); version != "" {
			return version
		}
	}

	// Default to HTML5 if we can't determine version
	return "HTML5 (assumed)"
}

// detectLoginForm checks if a form is likely a login form
func (a *MultipleUrlAnalyzer) detectLoginFormMulti(n *html.Node) bool {
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
// Modified for better concurrency handling
func (a *MultipleUrlAnalyzer) isLinkAccessibleMulti(ctx context.Context, link string) bool {
	// Create a client with a short timeout for link checking
	client := &http.Client{
		Timeout: 2 * time.Second, // Even shorter timeout for link checking in bulk
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

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

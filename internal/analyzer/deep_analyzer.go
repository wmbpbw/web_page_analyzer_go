package analyzer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/html"
	_ "log/slog"
)

// PageData contains detailed information about a webpage for deep analysis
type PageData struct {
	LoadTime      time.Duration
	TTFB          time.Duration
	Size          int64
	Resources     []ResourceData
	MetaTags      MetaTagsData
	Images        ImagesData
	Accessibility AccessibilityData
	Content       ContentData
	Security      SecurityData
	Mobile        MobileData
	Social        SocialData
	Technology    TechnologyData
	Media         MediaData
	Schema        SchemaData
	Cookies       CookiesData
	Links         LinksData
}

// ResourceData represents a resource on a webpage
type ResourceData struct {
	URL  string
	Type string
	Size int64
}

// MetaTagsData contains information about meta tags
type MetaTagsData struct {
	Title       string
	Description string
	Keywords    string
	Robots      string
	Canonical   string
}

// ImagesData contains information about images
type ImagesData struct {
	Total      int
	MissingAlt int
}

// AccessibilityData contains accessibility information
type AccessibilityData struct {
	AriaCount          int
	ContrastIssues     int
	KeyboardNavigation bool
}

// ContentData contains content analysis
type ContentData struct {
	WordCount        int
	KeywordDensity   map[string]float64
	ReadabilityScore float64
	TextToHTMLRatio  float64
}

// SecurityData contains security information
type SecurityData struct {
	CSPHeaders    bool
	XSSProtection bool
}

// MobileData contains mobile-friendliness information
type MobileData struct {
	HasViewport     bool
	IsResponsive    bool
	IsTouchFriendly bool
}

// SocialData contains social media integration information
type SocialData struct {
	HasOpenGraph     bool
	HasTwitterCards  bool
	SocialLinksCount int
}

// TechnologyData contains technology stack information
type TechnologyData struct {
	Server      string
	CMS         string
	Frameworks  []string
	Advertising []string
}

// MediaData contains media usage information
type MediaData struct {
	ImagesCount int
	VideoCount  int
	AudioCount  int
	TotalSize   int64
}

// SchemaData contains schema.org markup information
type SchemaData struct {
	HasSchema   bool
	SchemaTypes []string
	Format      string
}

// CookiesData contains cookie usage information
type CookiesData struct {
	TotalCount int
	FirstParty int
	ThirdParty int
	HasConsent bool
	MaxAgeDays int
}

// LinksData contains link analysis information
type LinksData struct {
	AnchorText  map[string]int
	NoFollow    int
	BrokenLinks int
	MaxDepth    int
}

// FetchPage fetches a webpage and performs deep analysis
func (a *Analyzer) FetchPage(ctx context.Context, urlStr string) (*PageData, error) {
	startTime := time.Now()

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
	a.logger.Info("Sending request for deep analysis", "url", urlStr)

	// Record time to first byte
	var ttfb time.Duration
	trace := &httptrace.ClientTrace{
		GotFirstResponseByte: func() {
			ttfb = time.Since(startTime)
		},
	}
	req = req.WithContext(httptrace.WithClientTrace(ctx, trace))

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	// Calculate load time
	loadTime := time.Since(startTime)

	// Read body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Get page size
	size := int64(len(body))

	// Parse HTML
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Initialize PageData
	pageData := &PageData{
		LoadTime:  loadTime,
		TTFB:      ttfb,
		Size:      size,
		Resources: []ResourceData{},
		Content: ContentData{
			KeywordDensity: make(map[string]float64),
		},
		Technology: TechnologyData{
			Frameworks:  []string{},
			Advertising: []string{},
		},
		Schema: SchemaData{
			SchemaTypes: []string{},
		},
		Links: LinksData{
			AnchorText: make(map[string]int),
		},
	}

	// Extract data from response headers
	pageData.Technology.Server = resp.Header.Get("Server")
	pageData.Security.CSPHeaders = resp.Header.Get("Content-Security-Policy") != ""
	pageData.Security.XSSProtection = resp.Header.Get("X-XSS-Protection") != ""

	// Extract cookies
	cookies := resp.Cookies()
	pageData.Cookies.TotalCount = len(cookies)

	// Process cookies
	for _, cookie := range cookies {
		if strings.Contains(cookie.Domain, parsedURL.Host) || cookie.Domain == "" {
			pageData.Cookies.FirstParty++
		} else {
			pageData.Cookies.ThirdParty++
		}

		// Check cookie max age
		if cookie.MaxAge > 0 {
			maxAgeDays := cookie.MaxAge / (60 * 60 * 24)
			if maxAgeDays > pageData.Cookies.MaxAgeDays {
				pageData.Cookies.MaxAgeDays = maxAgeDays
			}
		}
	}

	// Process the document
	a.processDocument(doc, parsedURL, pageData)

	// Perform readability analysis
	pageData.Content.ReadabilityScore = calculateReadabilityScore(string(body))

	// Calculate text to HTML ratio
	textContent := extractTextContent(doc)
	if size > 0 {
		pageData.Content.TextToHTMLRatio = float64(len(textContent)) / float64(size) * 100
	}

	// Calculate word count
	pageData.Content.WordCount = countWords(textContent)

	// Calculate keyword density
	pageData.Content.KeywordDensity = calculateKeywordDensity(textContent)

	return pageData, nil
}

// processDocument extracts data from the HTML document
func (a *Analyzer) processDocument(n *html.Node, baseURL *url.URL, data *PageData) {
	// Track counts
	var imagesCount, imagesWithoutAlt, videoCount, audioCount, ariaCount int
	var hasViewport, _, _, _, _ bool
	var _ string
	var schemaTypes []string
	var socialLinks int
	var noFollow int
	var anchorText = make(map[string]int)

	// Check for special meta tags, schema, etc.
	var processNode func(*html.Node, int)
	processNode = func(n *html.Node, depth int) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "meta":
				var name, property, content string
				for _, attr := range n.Attr {
					switch attr.Key {
					case "name":
						name = attr.Val
					case "property":
						property = attr.Val
					case "content":
						content = attr.Val
					}
				}

				// Process meta tags
				switch {
				case name == "description":
					data.MetaTags.Description = content
				case name == "keywords":
					data.MetaTags.Keywords = content
				case name == "robots":
					data.MetaTags.Robots = content
				case name == "viewport":
					hasViewport = true
					data.Mobile.HasViewport = true
					if strings.Contains(content, "width=device-width") {
						data.Mobile.IsResponsive = true
					}
				case property == "og:title" || property == "og:description" || property == "og:image":
					_ = true
					data.Social.HasOpenGraph = true
				case property == "twitter:card" || property == "twitter:title" || property == "twitter:description":
					_ = true
					data.Social.HasTwitterCards = true
				}

			case "link":
				var rel, href string
				for _, attr := range n.Attr {
					switch attr.Key {
					case "rel":
						rel = attr.Val
					case "href":
						href = attr.Val
					}
				}

				// Check for canonical link
				if rel == "canonical" {
					data.MetaTags.Canonical = href
				}

			case "title":
				if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
					data.MetaTags.Title = n.FirstChild.Data
				}

			case "img":
				imagesCount++
				hasAlt := false
				for _, attr := range n.Attr {
					if attr.Key == "alt" && attr.Val != "" {
						hasAlt = true
						break
					}
				}
				if !hasAlt {
					imagesWithoutAlt++
				}

			case "video":
				videoCount++

			case "audio":
				audioCount++

			case "script":
				var type_, src, innerHTML string
				for _, attr := range n.Attr {
					switch attr.Key {
					case "type":
						type_ = attr.Val
					case "src":
						src = attr.Val
					}
				}

				// Get inner HTML text
				if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
					innerHTML = n.FirstChild.Data
				}

				// Check for schema markup
				if type_ == "application/ld+json" {
					_ = true
					data.Schema.HasSchema = true
					data.Schema.Format = "JSON-LD"

					// Basic schema type detection
					if strings.Contains(innerHTML, "\"@type\":") {
						re := regexp.MustCompile(`"@type":\s*"([^"]+)"`)
						matches := re.FindAllStringSubmatch(innerHTML, -1)
						for _, match := range matches {
							if len(match) > 1 {
								schemaTypes = append(schemaTypes, match[1])
							}
						}
					}
				}

				// Detect frameworks and libraries
				for _, framework := range []string{"react", "angular", "vue", "jquery", "bootstrap"} {
					if strings.Contains(src, framework) || strings.Contains(innerHTML, framework) {
						data.Technology.Frameworks = append(data.Technology.Frameworks, framework)
					}
				}

				// Check for advertising
				for _, adNetwork := range []string{"adsense", "doubleclick", "adroll", "taboola", "outbrain"} {
					if strings.Contains(src, adNetwork) || strings.Contains(innerHTML, adNetwork) {
						data.Technology.Advertising = append(data.Technology.Advertising, adNetwork)
					}
				}

				// Check for cookie consent
				if strings.Contains(innerHTML, "cookie") &&
					(strings.Contains(innerHTML, "consent") || strings.Contains(innerHTML, "gdpr")) {
					_ = true
					data.Cookies.HasConsent = true
				}

			case "a":
				var href, rel, text string
				for _, attr := range n.Attr {
					switch attr.Key {
					case "href":
						href = attr.Val
					case "rel":
						rel = attr.Val
					}
				}

				// Extract anchor text
				if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
					text = strings.TrimSpace(n.FirstChild.Data)
					if text != "" {
						anchorText[text]++
					}
				}

				// Check for nofollow links
				if strings.Contains(rel, "nofollow") {
					noFollow++
				}

				// Check for social media links
				if href != "" {
					for _, socialDomain := range []string{"facebook.com", "twitter.com", "instagram.com", "linkedin.com", "youtube.com"} {
						if strings.Contains(href, socialDomain) {
							socialLinks++
							break
						}
					}
				}

			default:
				// Check for aria attributes
				for _, attr := range n.Attr {
					if strings.HasPrefix(attr.Key, "aria-") || attr.Key == "role" {
						ariaCount++
						break
					}
				}

				// Check for microdata schema
				for _, attr := range n.Attr {
					if strings.HasPrefix(attr.Key, "itemtype") || strings.HasPrefix(attr.Key, "itemprop") {
						_ = true
						data.Schema.HasSchema = true
						data.Schema.Format = "Microdata"

						if strings.HasPrefix(attr.Key, "itemtype") && strings.Contains(attr.Val, "schema.org/") {
							schemaType := strings.TrimPrefix(attr.Val, "https://schema.org/")
							schemaType = strings.TrimPrefix(schemaType, "http://schema.org/")
							schemaTypes = append(schemaTypes, schemaType)
						}
					}
				}
			}
		}

		// Process children
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			processNode(c, depth+1)
		}
	}

	// Start processing from the root
	processNode(n, 0)

	// Update PageData with collected information
	data.Images.Total = imagesCount
	data.Images.MissingAlt = imagesWithoutAlt
	data.Media.ImagesCount = imagesCount
	data.Media.VideoCount = videoCount
	data.Media.AudioCount = audioCount
	data.Accessibility.AriaCount = ariaCount
	data.Schema.SchemaTypes = schemaTypes
	data.Social.SocialLinksCount = socialLinks
	data.Links.NoFollow = noFollow
	data.Links.AnchorText = anchorText

	// Check for touch-friendly design (basic heuristic)
	data.Mobile.IsTouchFriendly = hasViewport

	// Estimate contrast issues (placeholder for a more sophisticated check)
	data.Accessibility.ContrastIssues = 0

	// Set keyboard navigation support (placeholder for a more accurate check)
	data.Accessibility.KeyboardNavigation = ariaCount > 0

	// Detect CMS (very basic detection)
	if strings.Contains(strings.ToLower(fmt.Sprintf("%s", n.Data)), "wordpress") {
		data.Technology.CMS = "WordPress"
	} else if strings.Contains(strings.ToLower(fmt.Sprintf("%s", n.Data)), "joomla") {
		data.Technology.CMS = "Joomla"
	} else if strings.Contains(strings.ToLower(fmt.Sprintf("%s", n.Data)), "drupal") {
		data.Technology.CMS = "Drupal"
	} else {
		data.Technology.CMS = "Unknown"
	}
}

// Helper functions

// extractTextContent extracts all text nodes from HTML
func extractTextContent(n *html.Node) string {
	var text string
	var extract func(*html.Node)
	extract = func(n *html.Node) {
		if n.Type == html.TextNode {
			text += " " + n.Data
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}
	extract(n)
	return strings.TrimSpace(text)
}

// countWords counts the number of words in a text
func countWords(text string) int {
	words := strings.Fields(text)
	return len(words)
}

// calculateReadabilityScore calculates Flesch-Kincaid readability score
func calculateReadabilityScore(text string) float64 {
	// Simplified implementation
	sentences := len(strings.Split(text, "."))
	words := len(strings.Fields(text))

	if sentences == 0 || words == 0 {
		return 0
	}

	// Average words per sentence
	wordsPerSentence := float64(words) / float64(sentences)

	// Simplified Flesch-Kincaid formula
	return 206.835 - (1.015 * wordsPerSentence)
}

// calculateKeywordDensity calculates keyword density
func calculateKeywordDensity(text string) map[string]float64 {
	words := strings.Fields(strings.ToLower(text))
	totalWords := len(words)

	if totalWords == 0 {
		return make(map[string]float64)
	}

	// Count word frequency
	wordCount := make(map[string]int)
	for _, word := range words {
		// Skip very short words and common stop words
		if len(word) < 3 || isStopWord(word) {
			continue
		}
		wordCount[word]++
	}

	// Calculate density
	density := make(map[string]float64)
	for word, count := range wordCount {
		density[word] = float64(count) / float64(totalWords) * 100
	}

	// Limit to top keywords
	const maxKeywords = 10
	if len(density) > maxKeywords {
		// Convert to slice for sorting
		type keywordDensity struct {
			keyword string
			density float64
		}

		keywordSlice := []keywordDensity{}
		for k, v := range density {
			keywordSlice = append(keywordSlice, keywordDensity{k, v})
		}

		// Sort by density (descending)
		sort.Slice(keywordSlice, func(i, j int) bool {
			return keywordSlice[i].density > keywordSlice[j].density
		})

		// Keep only top keywords
		result := make(map[string]float64)
		for i := 0; i < maxKeywords && i < len(keywordSlice); i++ {
			result[keywordSlice[i].keyword] = keywordSlice[i].density
		}
		return result
	}

	return density
}

// isStopWord checks if a word is a common stop word
func isStopWord(word string) bool {
	stopWords := map[string]bool{
		"a": true, "an": true, "the": true, "and": true, "or": true,
		"but": true, "is": true, "are": true, "was": true, "were": true,
		"be": true, "to": true, "of": true, "in": true, "that": true,
		"have": true, "it": true, "for": true, "on": true, "with": true,
	}
	return stopWords[word]
}

package internal

import (
	"bytes"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
)

// sanitizeHTML sanitizes HTML content or converts plain text newlines to HTML.
func sanitizeHTML(html string) string {
	// Check if the description contains HTML tags
	hasHTML := strings.Contains(html, "<") && strings.Contains(html, ">")

	if hasHTML {
		// Create custom policy for HTML content
		p := bluemonday.NewPolicy()

		// Allow safe formatting tags
		p.AllowElements("b", "i", "em", "strong", "br", "p", "ul", "ol", "li")

		// Sanitize and return
		return p.Sanitize(html)
	}

	// Plain text: convert newlines to <br> tags
	// First, trim leading/trailing whitespace
	text := strings.TrimSpace(html)

	// Replace multiple consecutive newlines with paragraph breaks
	text = strings.ReplaceAll(text, "\n\n", "</p><p>")

	// Replace single newlines with <br>
	text = strings.ReplaceAll(text, "\n", "<br>")

	// Wrap in paragraph tags
	text = "<p>" + text + "</p>"

	return text
}

// truncateHTML truncates HTML content to fit within maxLen characters while ensuring valid HTML structure.
// It appends "..." to indicate truncation. The function attempts to truncate intelligently, accounting for
// closing tags that need to be added to maintain valid HTML.
func truncateHTML(htmlContent string, maxLen int) string {
	// If already within limit, return as-is
	if len(htmlContent) <= maxLen {
		return htmlContent
	}

	ellipsis := "..."
	closingPTag := "</p>"
	commonSuffix := ellipsis + closingPTag
	targetLen := maxLen - len(commonSuffix)

	// Most content come as raw strings that we convert to basic HTML.
	truncated := htmlContent[:targetLen]
	validHTML := parseAndCloseHTMLTags(truncated + ellipsis)

	if len(validHTML) <= maxLen {
		return validHTML
	}

	// If the closing tags pushed us over, reduce by that amount.
	excess := len(validHTML) - targetLen
	if targetLen-excess > 0 {
		truncated = htmlContent[:targetLen-excess]
		validHTML = parseAndCloseHTMLTags(truncated + ellipsis)

		if len(validHTML) <= maxLen {
			return validHTML
		}
	}

	// Fallback to trimming significantly shorter than the maximum.
	safeLen := int(0.9 * float64(maxLen))
	if safeLen > len(htmlContent) {
		safeLen = len(htmlContent)
	}
	truncated = htmlContent[:safeLen]
	validHTML = parseAndCloseHTMLTags(truncated + ellipsis)

	return validHTML
}

// parseAndCloseHTMLTags parses an HTML fragment and returns valid HTML with all tags properly closed.
// The html.Parse function automatically closes any unclosed tags during parsing and rendering.
func parseAndCloseHTMLTags(htmlFragment string) string {
	doc, err := html.Parse(strings.NewReader(htmlFragment))
	if err != nil {
		// Can't parse - return as-is (shouldn't happen with sanitized input)
		return htmlFragment
	}

	// Find body element (parser wraps fragments in <html><body>)
	body := findBodyNode(doc)
	if body == nil {
		return htmlFragment
	}

	// Render just the body's children (excludes <html><body> wrapper)
	var buf bytes.Buffer
	for child := body.FirstChild; child != nil; child = child.NextSibling {
		html.Render(&buf, child)
	}

	return buf.String()
}

// findBodyNode recursively searches for the body element in an HTML node tree.
func findBodyNode(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.Data == "body" {
		return n
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if result := findBodyNode(child); result != nil {
			return result
		}
	}
	return nil
}

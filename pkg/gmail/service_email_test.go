// ABOUTME: Edge case tests for email HTML/MIME detection and composition
// ABOUTME: Tests encoding, header injection, malformed HTML, and special characters

package gmail

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsHTML_EdgeCases_HTMLComments(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected bool
	}{
		{
			name:     "HTML comment only",
			body:     "<!-- This is a comment -->",
			expected: false,
		},
		{
			name:     "HTML comment with HTML tag inside",
			body:     "<!-- <div>commented out</div> -->",
			expected: true,
		},
		{
			name:     "HTML with comment and actual HTML",
			body:     "<!-- comment --><div>Real content</div>",
			expected: true,
		},
		{
			name:     "Multiple HTML comments",
			body:     "<!-- comment 1 -->Text<!-- comment 2 -->",
			expected: false,
		},
		{
			name:     "Comment with newlines",
			body:     "<!--\n<div>test</div>\n-->",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isHTML(tt.body)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsHTML_EdgeCases_CDATA(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected bool
	}{
		{
			name:     "CDATA section only",
			body:     "<![CDATA[Some data]]>",
			expected: false,
		},
		{
			name:     "CDATA with HTML inside",
			body:     "<![CDATA[<div>test</div>]]>",
			expected: true,
		},
		{
			name:     "CDATA with actual HTML outside",
			body:     "<div><![CDATA[data]]></div>",
			expected: true,
		},
		{
			name:     "XML with CDATA",
			body:     "<xml><![CDATA[<script>alert('test')</script>]]></xml>",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isHTML(tt.body)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsHTML_EdgeCases_MixedContent(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected bool
	}{
		{
			name:     "Email signature with HTML",
			body:     "Best regards,\nJohn\n<br/>--<br/>Sent from my device",
			expected: true,
		},
		{
			name:     "Quoted text with HTML",
			body:     "> <div>Previous message</div>\nMy reply",
			expected: true,
		},
		{
			name:     "Mixed plain text and HTML entities",
			body:     "Hello &amp; goodbye < test >",
			expected: false,
		},
		{
			name:     "Markdown-like with HTML",
			body:     "# Header\n<p>But also HTML</p>",
			expected: true,
		},
		{
			name:     "Code snippet with HTML tags as strings",
			body:     "Use <p> tags for paragraphs",
			expected: true,
		},
		{
			name:     "Escaped HTML tags",
			body:     "&lt;div&gt;Not HTML&lt;/div&gt;",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isHTML(tt.body)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsHTML_EdgeCases_QuotedText(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected bool
	}{
		{
			name: "Email reply with HTML quote",
			body: `My new message

> <div>Original message</div>`,
			expected: true,
		},
		{
			name: "Multiple levels of quoted HTML",
			body: `Response
> > <p>Original</p>
> Reply`,
			expected: true,
		},
		{
			name: "Quoted plain text only",
			body: `Response
> Plain quoted text
> More quoted text`,
			expected: false,
		},
		{
			name:     "Gmail-style quote with HTML",
			body:     "Response\n\n<div class=\"gmail_quote\">On Mon, Jan 1<br><blockquote>Original</blockquote></div>",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isHTML(tt.body)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeHeader_EdgeCases_HeaderInjection(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Multiple newlines",
			input:    "test@example.com\n\n\nBcc: evil@attacker.com",
			expected: "test@example.comBcc: evil@attacker.com",
		},
		{
			name:     "Mixed line endings",
			input:    "test@example.com\r\n\nBcc: evil@attacker.com\rCc: another@attacker.com",
			expected: "test@example.comBcc: evil@attacker.comCc: another@attacker.com",
		},
		{
			name:     "Header injection in subject",
			input:    "Legitimate Subject\nBcc: attacker@evil.com\nContent-Type: text/html",
			expected: "Legitimate SubjectBcc: attacker@evil.comContent-Type: text/html",
		},
		{
			name:     "Null byte injection",
			input:    "test@example.com\x00Bcc: evil@attacker.com",
			expected: "test@example.com\x00Bcc: evil@attacker.com",
		},
		{
			name:     "Tab characters preserved",
			input:    "test@example.com\tlegit",
			expected: "test@example.com\tlegit",
		},
		{
			name:     "Unicode line separator (U+2028)",
			input:    "test@example.com\u2028Bcc: evil@attacker.com",
			expected: "test@example.com\u2028Bcc: evil@attacker.com",
		},
		{
			name:     "Unicode paragraph separator (U+2029)",
			input:    "test@example.com\u2029Bcc: evil@attacker.com",
			expected: "test@example.com\u2029Bcc: evil@attacker.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeHeader(tt.input)
			assert.Equal(t, tt.expected, result)
			assert.NotContains(t, result, "\r")
			assert.NotContains(t, result, "\n")
		})
	}
}

func TestBuildPlainTextMessage_EdgeCases_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name    string
		to      string
		subject string
		body    string
	}{
		{
			name:    "Unicode in all fields",
			to:      "test@例え.jp",
			subject: "Test 测试 тест",
			body:    "Body with emoji 🚀 and unicode 你好",
		},
		{
			name:    "Emojis in subject and body",
			to:      "test@example.com",
			subject: "🎉 Party Time 🎊",
			body:    "Let's celebrate! 🥳🎈🎁",
		},
		{
			name:    "RTL text",
			to:      "test@example.com",
			subject: "שלום",
			body:    "مرحبا العالم",
		},
		{
			name:    "Special punctuation",
			to:      "test@example.com",
			subject: "Test «quoted» text—dash",
			body:    "Ellipsis… and apostrophe's here",
		},
		{
			name:    "Newlines in body",
			to:      "test@example.com",
			subject: "Multi-line",
			body:    "Line 1\nLine 2\r\nLine 3\rLine 4",
		},
		{
			name:    "Very long subject",
			to:      "test@example.com",
			subject: strings.Repeat("A", 500),
			body:    "Short body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildPlainTextMessage(tt.to, "", "", tt.subject, tt.body, "", "")

			assert.Contains(t, result, "Content-Type: text/plain; charset=\"UTF-8\"")
			assert.Contains(t, result, "MIME-Version: 1.0")
			assert.Contains(t, result, tt.body)

			sanitizedTo := sanitizeHeader(tt.to)
			sanitizedSubject := sanitizeHeader(tt.subject)
			assert.Contains(t, result, sanitizedTo)
			assert.Contains(t, result, sanitizedSubject)

			lines := strings.Split(result, "\r\n")
			headerEnded := false
			for _, line := range lines {
				if line == "" {
					headerEnded = true
					continue
				}
				if !headerEnded {
					assert.NotContains(t, line, "\n", "Header line should not contain additional newlines")
				}
			}
		})
	}
}

func TestBuildHTMLMessage_EdgeCases_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name    string
		to      string
		subject string
		body    string
	}{
		{
			name:    "HTML with unicode",
			to:      "test@example.com",
			subject: "Unicode test 测试",
			body:    "<html><body><p>Hello 世界 🌍</p></body></html>",
		},
		{
			name:    "HTML with emojis",
			to:      "test@example.com",
			subject: "Emoji test",
			body:    "<html><body><h1>🚀 Launch!</h1><p>🎉🎊🥳</p></body></html>",
		},
		{
			name:    "HTML with RTL content",
			to:      "test@example.com",
			subject: "RTL test",
			body:    "<html><body dir=\"rtl\"><p>שלום עולם</p></body></html>",
		},
		{
			name:    "HTML with entities and unicode mixed",
			to:      "test@example.com",
			subject: "Mixed encoding",
			body:    "<html><body><p>&lt;tag&gt; and 中文 and &amp;</p></body></html>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildHTMLMessage(tt.to, "", "", tt.subject, tt.body, "", "")

			assert.Contains(t, result, "Content-Type: text/html; charset=\"UTF-8\"")
			assert.Contains(t, result, "MIME-Version: 1.0")
			assert.Contains(t, result, tt.body)

			sanitizedTo := sanitizeHeader(tt.to)
			sanitizedSubject := sanitizeHeader(tt.subject)
			assert.Contains(t, result, sanitizedTo)
			assert.Contains(t, result, sanitizedSubject)
		})
	}
}

func TestBuildPlainTextMessage_EdgeCases_LargeBody(t *testing.T) {
	tests := []struct {
		name     string
		bodySize int
	}{
		{
			name:     "1KB body",
			bodySize: 1024,
		},
		{
			name:     "10KB body",
			bodySize: 10 * 1024,
		},
		{
			name:     "100KB body",
			bodySize: 100 * 1024,
		},
		{
			name:     "1MB body",
			bodySize: 1024 * 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := strings.Repeat("A", tt.bodySize)
			to := "test@example.com"
			subject := "Large body test"

			result := buildPlainTextMessage(to, "", "", subject, body, "", "")

			assert.Contains(t, result, "Content-Type: text/plain; charset=\"UTF-8\"")
			assert.Contains(t, result, body)
			assert.Greater(t, len(result), tt.bodySize)
		})
	}
}

func TestBuildHTMLMessage_EdgeCases_MalformedHTML(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "Unclosed tags",
			body: "<html><body><p>Test<div>Unclosed",
		},
		{
			name: "Mismatched tags",
			body: "<html><body><p>Test</div></p></body></html>",
		},
		{
			name: "Invalid nesting",
			body: "<html><p><body>Wrong order</body></p></html>",
		},
		{
			name: "Missing closing tags",
			body: "<html><body><p>Test 1<p>Test 2<p>Test 3",
		},
		{
			name: "Self-closing non-void elements",
			body: "<html><body/><div/><p/>Content</html>",
		},
		{
			name: "HTML with script tags",
			body: "<html><body><script>alert('test');</script><p>Content</p></body></html>",
		},
		{
			name: "HTML with style tags",
			body: "<html><head><style>body { color: red; }</style></head><body>Test</body></html>",
		},
		{
			name: "HTML with inline styles",
			body: "<html><body><p style=\"color: red; font-size: 20px;\">Styled</p></body></html>",
		},
		{
			name: "HTML with attributes containing quotes",
			body: "<html><body><a href=\"#\" onclick=\"alert('test')\">Link</a></body></html>",
		},
		{
			name: "Empty HTML",
			body: "<html></html>",
		},
		{
			name: "HTML with only whitespace",
			body: "<html><body>   \n\t\r\n   </body></html>",
		},
		{
			name: "Deeply nested HTML",
			body: "<html><body><div><div><div><div><div><p>Deep</p></div></div></div></div></div></body></html>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			to := "test@example.com"
			subject := "Malformed HTML test"

			result := buildHTMLMessage(to, "", "", subject, tt.body, "", "")

			assert.Contains(t, result, "Content-Type: text/html; charset=\"UTF-8\"")
			assert.Contains(t, result, "MIME-Version: 1.0")
			assert.Contains(t, result, tt.body)
		})
	}
}

func TestBuildPlainTextMessage_EdgeCases_HeaderInjectionPrevention(t *testing.T) {
	tests := []struct {
		name            string
		to              string
		subject         string
		shouldNotAppear []string
	}{
		{
			name:            "Bcc injection in to field",
			to:              "test@example.com\nBcc: attacker@evil.com",
			subject:         "Normal subject",
			shouldNotAppear: []string{"\nBcc:", "\r\nBcc:"},
		},
		{
			name:            "Multiple header injection in subject",
			to:              "test@example.com",
			subject:         "Subject\nBcc: attacker@evil.com\nCc: another@evil.com",
			shouldNotAppear: []string{"\nBcc:", "\nCc:", "\r\nBcc:", "\r\nCc:"},
		},
		{
			name:            "Content-Type injection in subject",
			to:              "test@example.com",
			subject:         "Subject\nContent-Type: text/html",
			shouldNotAppear: []string{"\nContent-Type: text/html"},
		},
		{
			name:            "CRLF injection",
			to:              "test@example.com\r\nBcc: evil@attacker.com",
			subject:         "Test\r\nCc: another@attacker.com",
			shouldNotAppear: []string{"\r\nBcc:", "\r\nCc:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildPlainTextMessage(tt.to, "", "", tt.subject, "Test body", "", "")

			for _, forbidden := range tt.shouldNotAppear {
				assert.NotContains(t, result, forbidden,
					"Header injection was not properly prevented")
			}

			headerSection := strings.Split(result, "\r\n\r\n")[0]
			toLines := strings.Split(headerSection, "\r\n")
			for _, line := range toLines {
				assert.NotContains(t, line, "\n",
					"Header line contains unescaped newline")
				assert.NotContains(t, line, "\r",
					"Header line contains unescaped carriage return")
			}
		})
	}
}

func TestBuildHTMLMessage_EdgeCases_HeaderInjectionPrevention(t *testing.T) {
	tests := []struct {
		name            string
		to              string
		subject         string
		shouldNotAppear []string
	}{
		{
			name:            "Bcc injection in to field with HTML body",
			to:              "test@example.com\nBcc: attacker@evil.com",
			subject:         "Normal subject",
			shouldNotAppear: []string{"\nBcc:", "\r\nBcc:"},
		},
		{
			name:            "Content-Type override attempt",
			to:              "test@example.com",
			subject:         "Subject\nContent-Type: text/plain",
			shouldNotAppear: []string{"\nContent-Type: text/plain"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildHTMLMessage(tt.to, "", "", tt.subject, "<html><body>Test</body></html>", "", "")

			for _, forbidden := range tt.shouldNotAppear {
				assert.NotContains(t, result, forbidden,
					"Header injection was not properly prevented")
			}

			assert.Contains(t, result, "Content-Type: text/html; charset=\"UTF-8\"")
		})
	}
}

func TestIsHTML_EdgeCases_CaseSensitivity(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected bool
	}{
		{
			name:     "Mixed case HTML",
			body:     "<HtMl><BoDy>Test</BoDy></HtMl>",
			expected: true,
		},
		{
			name:     "Uppercase div",
			body:     "<DIV>Content</DIV>",
			expected: true,
		},
		{
			name:     "Mixed case br",
			body:     "Line 1<Br>Line 2",
			expected: true,
		},
		{
			name:     "Uppercase span",
			body:     "<SPAN>Text</SPAN>",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isHTML(tt.body)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildMessages_EdgeCases_EmptyFields(t *testing.T) {
	tests := []struct {
		name    string
		to      string
		subject string
		body    string
	}{
		{
			name:    "Empty body",
			to:      "test@example.com",
			subject: "Subject",
			body:    "",
		},
		{
			name:    "Whitespace only body",
			to:      "test@example.com",
			subject: "Subject",
			body:    "   \n\t\r\n   ",
		},
		{
			name:    "Single space body",
			to:      "test@example.com",
			subject: "Subject",
			body:    " ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+" (plain text)", func(t *testing.T) {
			result := buildPlainTextMessage(tt.to, "", "", tt.subject, tt.body, "", "")
			assert.Contains(t, result, "MIME-Version: 1.0")
			assert.Contains(t, result, "Content-Type: text/plain")
		})

		t.Run(tt.name+" (HTML)", func(t *testing.T) {
			htmlBody := "<html><body>" + tt.body + "</body></html>"
			result := buildHTMLMessage(tt.to, "", "", tt.subject, htmlBody, "", "")
			assert.Contains(t, result, "MIME-Version: 1.0")
			assert.Contains(t, result, "Content-Type: text/html")
		})
	}
}

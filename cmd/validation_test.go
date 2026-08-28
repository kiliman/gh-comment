package cmd

import (
	"strings"
	"testing"
)

func TestValidateCommentBodySecurity(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid comment body with markdown",
			body:    "This is a normal comment with markdown **bold** and `code`",
			wantErr: false,
		},
		{
			name:    "comment with safe HTML entities",
			body:    "Testing &lt;script&gt; and &amp; entities",
			wantErr: false,
		},
		{
			name:    "github mentions and links",
			body:    "@user mentioned in #123 and linked to PR #456",
			wantErr: false,
		},
		{
			name:        "dangerous script tag",
			body:        "This comment has a <script>alert('xss')</script> tag",
			wantErr:     true,
			errContains: "dangerous HTML tags detected",
		},
		{
			name:        "iframe tag",
			body:        "Check this out: <iframe src='evil.com'></iframe>",
			wantErr:     true,
			errContains: "dangerous HTML tags detected",
		},
		{
			name:        "object tag",
			body:        "Embedded content: <object data='malware.exe'></object>",
			wantErr:     true,
			errContains: "dangerous HTML tags detected",
		},
		{
			name:        "form tag",
			body:        "Login here: <form><input type='password'></form>",
			wantErr:     true,
			errContains: "dangerous HTML tags detected",
		},
		{
			name:        "javascript in href",
			body:        "Click <a href='javascript:alert(1)'>here</a>",
			wantErr:     true,
			errContains: "JavaScript content detected",
		},
		{
			name:        "onclick handler",
			body:        "Click <button onclick='doEvil()'>here</button>",
			wantErr:     true,
			errContains: "dangerous attributes detected",
		},
		{
			name:        "onload handler",
			body:        "Image: <img onload='steal()' src='pic.jpg'>",
			wantErr:     true,
			errContains: "dangerous attributes detected",
		},
		{
			name:        "javascript protocol",
			body:        "Link: <a href='javascript:void(0)'>test</a>",
			wantErr:     true,
			errContains: "JavaScript content detected",
		},
		{
			name:        "case insensitive script tag",
			body:        "Sneaky: <ScRiPt>alert('case insensitive')</ScRiPt>",
			wantErr:     true,
			errContains: "dangerous HTML tags detected",
		},
		{
			name:        "script tag with whitespace",
			body:        "Spaced: < script >alert('whitespace')</ script >",
			wantErr:     true,
			errContains: "dangerous HTML tags detected",
		},
		{
			name:        "vbscript attribute",
			body:        "VB: <div vbscript='MsgBox'>content</div>",
			wantErr:     true,
			errContains: "dangerous attributes detected",
		},
		{
			name:        "data attribute with javascript",
			body:        "Data: <img data='javascript:alert()' src='test.jpg'>",
			wantErr:     true,
			errContains: "JavaScript content detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCommentBody(tt.body)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateCommentBody() expected error but got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("validateCommentBody() error = %v, want to contain %v", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("validateCommentBody() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidateCommentSecurity(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantErr     bool
		errContains string
	}{
		{
			name:    "safe markdown content",
			body:    "# Header\n\n**Bold** and *italic* text with `code`",
			wantErr: false,
		},
		{
			name:    "safe HTML entities",
			body:    "Less than &lt; and greater than &gt; symbols",
			wantErr: false,
		},
		{
			name:    "github mention and issue links",
			body:    "@user mentioned in #123 and linked to PR #456",
			wantErr: false,
		},
		{
			name:        "script tag attack",
			body:        "<script>document.cookie='stolen'</script>",
			wantErr:     true,
			errContains: "🔒 security validation failed",
		},
		{
			name:        "iframe injection",
			body:        "<iframe src='data:text/html,<script>alert(1)</script>'></iframe>",
			wantErr:     true,
			errContains: "dangerous HTML tags detected",
		},
		{
			name:        "embed tag",
			body:        "<embed src='malicious.swf' type='application/x-shockwave-flash'>",
			wantErr:     true,
			errContains: "dangerous HTML tags detected",
		},
		{
			name:        "meta refresh redirect",
			body:        "<meta http-equiv='refresh' content='0;url=evil.com'>",
			wantErr:     true,
			errContains: "dangerous HTML tags detected",
		},
		{
			name:        "link to external stylesheet",
			body:        "<link rel='stylesheet' href='http://evil.com/steal.css'>",
			wantErr:     true,
			errContains: "dangerous HTML tags detected",
		},
		{
			name:        "event handler in any tag",
			body:        "<div onmouseover='evil()'>Hover me</div>",
			wantErr:     true,
			errContains: "dangerous attributes detected",
		},
		{
			name:        "multiple dangerous elements",
			body:        "Start <script>evil()</script> middle <iframe src='bad'></iframe> end",
			wantErr:     true,
			errContains: "dangerous HTML tags detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCommentSecurity(tt.body)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateCommentSecurity() expected error but got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("validateCommentSecurity() error = %v, want to contain %v", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("validateCommentSecurity() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidateRepositoryAccess(t *testing.T) {
	tests := []struct {
		name        string
		repo        string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid repository",
			repo:    "owner/repo",
			wantErr: false,
		},
		{
			name:    "repository with numbers",
			repo:    "user123/repo456",
			wantErr: false,
		},
		{
			name:    "repository with hyphens and underscores",
			repo:    "my-org/my_repo",
			wantErr: false,
		},
		{
			name:    "repository with dots",
			repo:    "org.name/repo.name",
			wantErr: false,
		},
		{
			name:        "invalid format - no slash",
			repo:        "invalidrepo",
			wantErr:     true,
			errContains: "must be 'owner/repo'",
		},
		{
			name:        "invalid format - multiple slashes",
			repo:        "owner/repo/extra",
			wantErr:     true,
			errContains: "must be 'owner/repo'",
		},
		{
			name:        "directory traversal dots in owner",
			repo:        "evil..owner/repo",
			wantErr:     true,
			errContains: "path traversal",
		},
		{
			name:        "directory traversal dots in repo name",
			repo:        "owner/repo..evil",
			wantErr:     true,
			errContains: "path traversal",
		},
		{
			name:        "invalid characters",
			repo:        "owner$/repo%",
			wantErr:     true,
			errContains: "invalid characters",
		},
		{
			name:        "empty owner",
			repo:        "/repo",
			wantErr:     true,
			errContains: "owner and repository name cannot be empty",
		},
		{
			name:        "empty repo name",
			repo:        "owner/",
			wantErr:     true,
			errContains: "owner and repository name cannot be empty",
		},
		{
			name:        "reserved name CON",
			repo:        "CON/repo",
			wantErr:     true,
			errContains: "reserved system names",
		},
		{
			name:        "reserved name PRN",
			repo:        "owner/PRN",
			wantErr:     true,
			errContains: "reserved system names",
		},
		{
			name:        "reserved name case insensitive",
			repo:        "owner/con",
			wantErr:     true,
			errContains: "reserved system names",
		},
		{
			name:        "reserved name LPT1",
			repo:        "lpt1/repo",
			wantErr:     true,
			errContains: "reserved system names",
		},
		{
			name:        "owner too long",
			repo:        strings.Repeat("a", MaxAuthorLength+1) + "/repo",
			wantErr:     true,
			errContains: "repository owner too long",
		},
		{
			name:        "repo name causing total length to exceed",
			repo:        "owner/" + strings.Repeat("b", MaxRepoNameLength+1),
			wantErr:     true,
			errContains: "repository name too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRepositoryAccess(tt.repo)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateRepositoryAccess() expected error but got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("validateRepositoryAccess() error = %v, want to contain %v", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("validateRepositoryAccess() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidateCommentThreadDepth(t *testing.T) {
	tests := []struct {
		name        string
		depth       int
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid depth 0",
			depth:   0,
			wantErr: false,
		},
		{
			name:    "valid depth 1",
			depth:   1,
			wantErr: false,
		},
		{
			name:    "valid depth at maximum",
			depth:   MaxCommentThreadDepth,
			wantErr: false,
		},
		{
			name:        "negative depth",
			depth:       -1,
			wantErr:     true,
			errContains: "must be non-negative",
		},
		{
			name:        "depth exceeds maximum",
			depth:       MaxCommentThreadDepth + 1,
			wantErr:     true,
			errContains: "🚫 access validation failed",
		},
		{
			name:        "extremely deep nesting",
			depth:       100,
			wantErr:     true,
			errContains: "performance issues",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCommentThreadDepth(tt.depth)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateCommentThreadDepth() expected error but got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("validateCommentThreadDepth() error = %v, want to contain %v", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("validateCommentThreadDepth() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidationErrorFormatters(t *testing.T) {
	tests := []struct {
		name     string
		function func() error
		want     string
	}{
		{
			name: "formatSecurityValidationError",
			function: func() error {
				return formatSecurityValidationError("test field", "test issue", "test guidance")
			},
			want: "🔒 security validation failed for test field: test issue\n\n💡 Guidance: test guidance",
		},
		{
			name: "formatAccessValidationError",
			function: func() error {
				return formatAccessValidationError("test resource", "test action", "test guidance")
			},
			want: "🚫 access validation failed for test resource: cannot test action\n\n💡 Guidance: test guidance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.function()
			if err == nil {
				t.Errorf("Expected error but got nil")
				return
			}
			if err.Error() != tt.want {
				t.Errorf("Error message mismatch:\nGot:  %q\nWant: %q", err.Error(), tt.want)
			}
		})
	}
}

func TestValidationResult(t *testing.T) {
	tests := []struct {
		name           string
		validationFunc func() ValidationResult
		wantField      string
		wantValid      bool
		wantError      bool
	}{
		{
			name: "valid field validation",
			validationFunc: createFieldValidator("test-field", "test-value", func() error {
				return nil
			}),
			wantField: "test-field",
			wantValid: true,
			wantError: false,
		},
		{
			name: "invalid field validation",
			validationFunc: createFieldValidator("test-field", "test-value", func() error {
				return formatValidationError("test-field", "test-value", "must be valid")
			}),
			wantField: "test-field",
			wantValid: false,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.validationFunc()

			if result.Field != tt.wantField {
				t.Errorf("ValidationResult.Field = %v, want %v", result.Field, tt.wantField)
			}
			if result.Valid != tt.wantValid {
				t.Errorf("ValidationResult.Valid = %v, want %v", result.Valid, tt.wantValid)
			}
			if (result.Error != nil) != tt.wantError {
				t.Errorf("ValidationResult.Error = %v, wantError %v", result.Error, tt.wantError)
			}
		})
	}
}

func TestValidateMultipleFields(t *testing.T) {
	tests := []struct {
		name           string
		validations    []func() ValidationResult
		wantResultsLen int
		wantAllValid   bool
	}{
		{
			name: "all validations pass",
			validations: []func() ValidationResult{
				createFieldValidator("field1", "value1", func() error { return nil }),
				createFieldValidator("field2", "value2", func() error { return nil }),
			},
			wantResultsLen: 2,
			wantAllValid:   true,
		},
		{
			name: "some validations fail",
			validations: []func() ValidationResult{
				createFieldValidator("field1", "value1", func() error { return nil }),
				createFieldValidator("field2", "value2", func() error {
					return formatValidationError("field2", "value2", "invalid")
				}),
			},
			wantResultsLen: 2,
			wantAllValid:   false,
		},
		{
			name:           "empty validations",
			validations:    []func() ValidationResult{},
			wantResultsLen: 0,
			wantAllValid:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := validateMultipleFields(tt.validations)

			if len(results) != tt.wantResultsLen {
				t.Errorf("validateMultipleFields() returned %d results, want %d", len(results), tt.wantResultsLen)
			}

			allValid := true
			for _, result := range results {
				if !result.Valid {
					allValid = false
					break
				}
			}

			if allValid != tt.wantAllValid {
				t.Errorf("validateMultipleFields() allValid = %v, want %v", allValid, tt.wantAllValid)
			}
		})
	}
}

// A code review comment's whole job is quoting source. These are the shapes that
// used to be refused outright: event props, a `data` assignment, and form markup.
func TestValidateCommentBodyAllowsQuotedCode(t *testing.T) {
	allowed := []struct {
		name string
		body string
	}{
		{"inline event prop", "The `onClose={() => setOpen(false)}` handler never fires."},
		{"fenced jsx", "Consider:\n\n```tsx\n<WebDnsRecordsSlideOver\n  onClose={() => setOpen(false)}\n/>\n```\n\nThat drops the prop."},
		{"tilde fence", "~~~js\nel.onclick = handler\n~~~"},
		{"data assignment in prose", "This shadows the outer scope: const data = await res.json()"},
		{"fenced form markup", "```html\n<form><input type='password'></form>\n```"},
	}

	for _, tt := range allowed {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateCommentBody(tt.body); err != nil {
				t.Errorf("validateCommentBody() rejected quoted code: %v", err)
			}
		})
	}

	// Stripping code must not become a bypass: raw HTML outside a fence is still
	// refused, and a fence that is never closed strips nothing.
	refused := []struct {
		name string
		body string
	}{
		{"handler outside code", "Click <button onclick='doEvil()'>here</button>"},
		{"unterminated fence", "```\n<script>alert('xss')</script>"},
		{"prose after a closed fence", "```js\nok()\n```\nand then <script>alert('xss')</script>"},
	}

	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateCommentBody(tt.body); err == nil {
				t.Error("validateCommentBody() accepted raw HTML outside code")
			}
		})
	}
}

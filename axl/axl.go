package axl

import (
	"crypto/tls"
	"net/http"
	"encoding/xml"
	"fmt"
	"io"
	"text/template"
	"strings"
)

const (
	soapEnvelope = `<s:Envelope xmlns:s="https://schemas.xmlsoap.org/soap/envelope/" xmlns:n="https://www.cisco.com/AXL/API/{{ .Version }}"><s:Header/><s:Body>{{ .Body }}</s:Body></s:Envelope>`
)

type AXLClient struct {
	httpClient *http.Client
	cucm string
	username string
	password string
	schemaVersion string
	insecureSkipVerify bool
	jsessionidcookie *http.Cookie
}

func NewClient(cucm string) *AXLClient {
	return &AXLClient{cucm: cucm}
}

func (c *AXLClient) SetAuthentication(username, password string) *AXLClient {
	c.username = username
	c.password = password
	return c
}

func (c *AXLClient) SetSchemaVersion(v string) *AXLClient {
	c.schemaVersion = v
	return c
}

func (c *AXLClient) SetInsecureSkipVerify(b bool) *AXLClient {
	c.insecureSkipVerify = b
	return c
}

func (c *AXLClient) AXLRequest(r io.Reader) {
	if c.httpClient == nil {
		c.httpClient = &http.Client{
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: 15 * time.Second, // time spend establishing TCP connection
					KeepAlive: 15 * time.Second, // interval between keep-alive probes
				}).DialContext,
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: c.insecureSkipVerify,
				},
				TLSHandshakeTimeout: 10 * time.Second,
				ResponseHeaderTimeout: 15 * time.Second,
			},
		}
	}
	rPipe, wPipe := io.Pipe()
	go func() {
		enc := NewEncoder(wPipe)
		enc.SchemaVersion = c.schemaVersion
		if err := enc.Encode(r); err != nil {
			log.Fatalf("error encoding AXL: %v", err)
		}
	}

	req, _  = http.NewRequest("POST", "https://" + c.cucm + "/axl/", rPipe)
	if c.jsessionidcookie == nil {
		req.SetBasicAuth(c.Username, c.Password)
	} else {
		req.AddCookie(c.jsessionidcookie)
	}
	req.Header.Add("Content-Type", "text/xml")
	req.Header.Add("SOAPAction", fmt.Sprintf("\"CUCM:DB ver=%s %s\"", c.schemaVersion, method))
	resp, err := c.httpClient.Do(req)
type AXLEncoder struct {
	SchemaVersion string // AXL schema
	w io.Writer
	headerWritten bool
	tmpl *template.Template
	axlMethod string // detected by EncodeToken
}

func (a *AXLEncoder) Encode(r io.Reader) error {
	if a.SchemaVersion == "" {
		panic("AXL SchemaVersion not specified.")
	}

	var buf strings.Builder

	if err := removeWS(&buf, r); err != nil {
		return err
	}

	data := struct{
		Version string
		Body string
	}{
		Version: a.SchemaVersion,
		Body: buf.String(),
	}
	if err := a.tmpl.Execute(a.w, data); err != nil {
		return err
	}
	return nil
}

func NewEncoder(w io.Writer) *AXLEncoder {
	return &AXLEncoder{
		w: w,
		tmpl: template.Must(template.New("axl").Parse(soapEnvelope)),
	}
}

func removeWS(buf *strings.Builder, r io.Reader) error {
	dec := xml.NewDecoder(r)
	enc := NewSkipWSEncoder(buf)
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to decode XML token: %w", err)
		}

		if err := enc.EncodeToken(tok); err != nil {
			return fmt.Errorf("failed to encode XML token: %w", err)
		}
	}

	if err := enc.Flush(); err != nil {
		return fmt.Errorf("failed to flush XML encoder: %w", err)
	}

	return nil
}

type SkipWSEncoder struct {
	*xml.Encoder

	startElementLastSeen bool
	level int // nesting level to decide if top-level namespace prefix is needed
	charData xml.CharData
	topLevelElement string
}

func NewSkipWSEncoder(w io.Writer) *SkipWSEncoder {
	return &SkipWSEncoder{
		Encoder: xml.NewEncoder(w),
	}
}

// normalizePrefix returns s with the static namespace prefix "n:".
// If a prefix was used before it will be replaced. It also returns
// the tag level name so it can be used as AXL method.
func normalizePrefix(s string) (string, string) {
	s2 := strings.SplitN(s, ":", 2)
	return fmt.Sprintf("n:%v", s2[len(s2)-1]), s2[len(s2)-1]
}

// EncodeToken skips insignificant whitespace and adds the
// namespace prefix "n:" to the top level XML element. XML
// Comments are ignored (not encoded).
func (swe *SkipWSEncoder) EncodeToken(tok xml.Token) error {
	switch tok := tok.(type) {
	case xml.CharData:
		swe.charData = tok.Copy()
		return nil
	case xml.StartElement:
		swe.startElementLastSeen = true
		swe.charData = nil
		swe.level += 1
		if swe.level == 1 {
			tok.Name.Local, swe.topLevelElement = normalizePrefix(tok.Name.Local)
			return swe.Encoder.EncodeToken(tok)
		}
	case xml.EndElement:
		if swe.startElementLastSeen && len(swe.charData) > 0 {
			if err := swe.Encoder.EncodeToken(swe.charData); err != nil {
				return err
			}
		}
		swe.startElementLastSeen = false
		if swe.level == 1 {
			tok.Name.Local, _ = normalizePrefix(tok.Name.Local)
			return swe.Encoder.EncodeToken(tok)
		}
		swe.level -= 1
	case xml.Comment:
		return nil // filter XML comments, if present
	}
	return swe.Encoder.EncodeToken(tok)
}


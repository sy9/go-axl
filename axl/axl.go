package axl

import (
	"bytes"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"text/template"
	"time"
)

const (
	soapEnvelope = `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" xmlns:n="http://www.cisco.com/AXL/API/{{ .Version }}"><s:Header/><s:Body>{{ .Body }}</s:Body></s:Envelope>`
)

// AXLResponse contains the response
type AXLResponse struct {
	RawBody []byte
}

//<?xml version='1.0' encoding='UTF-8'?><soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"><soapenv:Body><soapenv:Fault><faultcode>soapenv:Client</faultcode><faultstring>Could not insert new row - duplicate value in a UNIQUE INDEX column (Unique Index:).</faultstring><detail><axlError><axlcode>-239</axlcode><axlmessage>Could not insert new row - duplicate value in a UNIQUE INDEX column (Unique Index:).</axlmessage><request>addRoutePartition</request></axlError></detail></soapenv:Fault></soapenv:Body></soapenv:Envelope>

// AXLError contains AXL error details
type AXLError struct {
	Faultcode       string `xml:"Body>Fault>faultcode"`
	Faultstring     string `xml:"Body>Fault>faultstring"`
	AXLErrorCode    int    `xml:"Body>Fault>detail>axlError>axlcode"`
	AXLErrorMessage string `xml:"Body>Fault>detail>axlError>axlmessage"`
	AXLRequest      string `xml:"Body>Fault>detail>axlError>request"`
}

func (a *AXLError) Error() string {
	return fmt.Sprintf("AXL msg: %q, error code: %d", a.AXLErrorMessage, a.AXLErrorCode)
}

// AXLClient issues SOAP/AXL requests to CUCM and returns the response.
type AXLClient struct {
	httpClient         *http.Client
	cucm               string
	username           string
	password           string
	schemaVersion      string
	insecureSkipVerify bool
	jsessionidcookie   *http.Cookie
	buf                *bytes.Buffer
	axlMethod          string
}

// NewClient creates a new AXLClient. The CUCM first node FQDN/IP is necessary
// for initialization.
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

// AXLRequest reads XML data from the specified reader and issues
// an AXL request to CUCM. The XML data must not contain any SOAP
// headers. Insignificant whitespace will be removed. The correct
// AXL method will be inferred from the included top-level element.
func (c *AXLClient) AXLRequest(r io.Reader) (*AXLResponse, error) {
	if c.httpClient == nil {
		c.createClient()
	}

	c.buf = new(bytes.Buffer)
	enc := NewEncoder(c.buf)
	// let the encoder know the AXL schema version so it can be included
	// in the SOAP header
	enc.SchemaVersion = c.schemaVersion

	// Encode removes insignificant whitespace and checks
	// if XML syntax is correct. It also reads the AXL method from
	// the top-level XML element
	if err := enc.Encode(r); err != nil {
		return nil, fmt.Errorf("error encoding AXL: %w", err)
	}

	// save AXL method so it can be used inside SOAPAction HTTP header
	c.axlMethod = enc.axlMethod

	req := c.createRequest()

	return c.handleResult(c.httpClient.Do(req))
}

func isXML(s string) bool {
	return strings.Contains(s, "/xml") // can be application/xml or text/xml
}

func (c *AXLClient) handleResult(resp *http.Response, err error) (*AXLResponse, error) {
	if err != nil {
		return nil, fmt.Errorf("AXL request failed: %w", err)
	}
	defer resp.Body.Close()

	// here we reverse the logic of "happy path" and establish an "unhappy path"
	if resp.StatusCode == 200 {
		var axlResponse AXLResponse
		axlResponse.RawBody, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("Error reading AXL response body: %w", err)
		}
		return &axlResponse, nil
	}

	if isXML(resp.Header.Get("Content-Type")) {
		var axlError AXLError
		err = xml.NewDecoder(resp.Body).Decode(&axlError)
		if err != nil {
			return nil, fmt.Errorf("Error decoding AXL error response: %w", err)
		}
		return nil, &axlError
	}
	return nil, fmt.Errorf("AXL request failed with status code %v, message %q", resp.StatusCode, resp.Status)
}

func (c *AXLClient) createRequest() *http.Request {
	req, _ := http.NewRequest("POST", "https://"+c.cucm+"/axl/", c.buf)
	if c.jsessionidcookie == nil {
		req.SetBasicAuth(c.username, c.password)
	} else {
		req.AddCookie(c.jsessionidcookie)
	}
	req.Header.Add("Content-Type", "text/xml")
	req.Header.Add("SOAPAction", fmt.Sprintf("\"CUCM:DB ver=%s %s\"", c.schemaVersion, c.axlMethod))
	return req
}

func (c *AXLClient) createClient() {
	c.httpClient = &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   15 * time.Second, // time spend establishing TCP connection
				KeepAlive: 15 * time.Second, // interval between keep-alive probes
			}).DialContext,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: c.insecureSkipVerify,
			},
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
		},
	}
}

type AXLEncoder struct {
	SchemaVersion string // AXL schema
	w             io.Writer
	headerWritten bool
	tmpl          *template.Template
	axlMethod     string // detected by EncodeToken
}

func NewEncoder(w io.Writer) *AXLEncoder {
	return &AXLEncoder{
		w:    w,
		tmpl: template.Must(template.New("axl").Parse(soapEnvelope)),
	}
}

func (a *AXLEncoder) Encode(r io.Reader) error {
	if a.SchemaVersion == "" {
		panic("AXL SchemaVersion not specified.")
	}

	var buf strings.Builder

	top, err := removeWS(&buf, r)
	if err != nil {
		return err
	}

	data := struct {
		Version string
		Body    string
	}{
		Version: a.SchemaVersion,
		Body:    buf.String(),
	}
	a.axlMethod = top // save top level XML element
	if a.tmpl == nil {
		panic("a.tmpl == nil")
	}
	if a.w == nil {
		panic("a.w == nil")
	}
	if err := a.tmpl.Execute(a.w, data); err != nil {
		return err
	}
	return nil
}

func removeWS(buf io.Writer, r io.Reader) (string, error) {
	dec := xml.NewDecoder(r)
	enc := NewSkipWSEncoder(buf)
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("failed to decode XML token: %w", err)
		}

		if err := enc.EncodeToken(tok); err != nil {
			return "", fmt.Errorf("failed to encode XML token: %w", err)
		}
	}

	if err := enc.Flush(); err != nil {
		return "", fmt.Errorf("failed to flush XML encoder: %w", err)
	}

	return enc.topLevelElement, nil
}

type SkipWSEncoder struct {
	*xml.Encoder

	startElementLastSeen bool
	level                int // nesting level to decide if top-level namespace prefix is needed
	charData             xml.CharData
	topLevelElement      string
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

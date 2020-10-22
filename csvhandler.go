package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"text/template"
)

// CSVHandler is saving CSV records and can execute a template
// for each record.
type CSVHandler struct {
	records      [][]string
	index        int   // row index
	bulkMode     bool  // used for displaying correct error message when var index is out of range
	logIndexList []int // which CSV columns to include in log output
	logRun       bool  // initialize logIndexList
	initialized  bool  // set to true after first call to Do()
	tmpl         *template.Template
}

func NewCSVHandler(body []byte) (*CSVHandler, error) {
	var csvHandler CSVHandler

	funcMap := template.FuncMap{
		"var":    csvHandler.varFunc(),
		"varlog": csvHandler.varLogFunc(),
	}

	tmpl, err := template.New("axl").Funcs(funcMap).Parse(string(body))
	if err != nil {
		return nil, fmt.Errorf("error parsing XML template: %w", err)
	}
	csvHandler.tmpl = tmpl
	csvHandler.records = [][]string{[]string{}} // one empty record
	csvHandler.index = -1

	return &csvHandler, nil
}

// SetRecords stores CSV records internally
func (c *CSVHandler) SetRecords(records [][]string) {
	c.records = records
	c.bulkMode = true
}

// Next must be called before Do(). It returns true if a record is available.
func (c *CSVHandler) Next() bool {
	c.index += 1
	if c.index == len(c.records) {
		return false
	}
	return true
}

// Do executes the template with the current record.
func (c *CSVHandler) Do(w io.Writer) error {
	if !c.initialized {
		if err := c.logRunFunc(); err != nil { // look for {{varlog n}} statements and add n to list of log indices
			return err
		}
		c.initialized = true
	}
	return c.tmpl.Execute(w, nil)
}

func (c *CSVHandler) LogIndexAndItem() string {
	if c.bulkMode {
		return fmt.Sprintf("Line: %d, Item: %q", c.index+1, c.getLogElement())
	} else {
		return "" // no prefix if not in bulk mode
	}
}

// VarFunc returns a function that is used inside a template to include
// content from CSV files. It refers to the column index (starting at 0)
// e.g. {{ var 0 }} for the first column.
func (c *CSVHandler) varFunc() func(int) (string, error) {
	return func(i int) (string, error) {
		return varFunc(i, c.records[c.index], c.bulkMode)
	}
}

// VarLogFunc is the same as VarFunc, except that content from these columns
// are logged. It is called with {{ varlog n }} in the template.
func (c *CSVHandler) varLogFunc() func(int) (string, error) {
	return func(i int) (string, error) {
		if c.logRun && !contains(c.logIndexList, i) {
			c.logIndexList = append(c.logIndexList, i)
		}
		return varFunc(i, c.records[c.index], c.bulkMode)
	}
}

func (c *CSVHandler) logRunFunc() error {
	c.logRun = true
	err := c.tmpl.Execute(ioutil.Discard, nil) // we just need the side-effects (i.e. calling varlog)
	if err != nil {
		return fmt.Errorf("error during template log-run: %w", err)
	}
	if c.logIndexList == nil {
		c.logIndexList = []int{0} // default output first CSV column
	}
	c.logRun = false
	return nil
}

func (c *CSVHandler) getLogElement() string {
	if c.bulkMode != true {
		return "<static>"
	}
	var b strings.Builder
	first := true
	for _, i := range c.logIndexList {
		if !first {
			fmt.Fprintf(&b, ",%s", c.records[c.index][i])
		} else {
			fmt.Fprintf(&b, "%s", c.records[c.index][i])
			first = false
		}
	}
	return b.String()
}

func varFunc(index int, record []string, bulkMode bool) (string, error) {
	if index > len(record)-1 {
		if bulkMode {
			return "", fmt.Errorf("var index out of range (%d) (hint: first CSV column has index 0)", index)
		} else {
			return "", fmt.Errorf("var function in XML template is not allowed without CSV file")
		}
	}
	return xmlEscape(record[index])
}

func xmlEscape(s string) (string, error) {
	var b strings.Builder
	err := xml.EscapeText(&b, []byte(s))
	if err != nil {
		return "", err
	}
	return b.String(), nil
}

func contains(s []int, e int) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

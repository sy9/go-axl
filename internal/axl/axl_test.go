package axl

import (
	"strings"
	"testing"
)

func TestRemoveWS(t *testing.T) {
	tests := map[string]struct {
		input string
		want  string
		top   string
	}{
		"top level element only":                  {input: "<addRoutePartition></addRoutePartition>", want: "<n:addRoutePartition></n:addRoutePartition>", top: "addRoutePartition"},
		"top level element only w/ empty content": {input: "<addRoutePartition> </addRoutePartition>", want: "<n:addRoutePartition> </n:addRoutePartition>", top: "addRoutePartition"},
		"empty sub expansion and WS removal":      {input: "<addRoutePartition>   \n   <empty/> asdf  </addRoutePartition>", want: "<n:addRoutePartition><empty></empty></n:addRoutePartition>", top: "addRoutePartition"},
		"ignore XML comment":                      {input: "<addRoutePartition>   \n   <empty/> asadf<test><!-- a comment --></test>  </addRoutePartition>", want: "<n:addRoutePartition><empty></empty><test></test></n:addRoutePartition>", top: "addRoutePartition"},
	}

	for name, tc := range tests {
		var buf strings.Builder
		top, err := removeWS(&buf, strings.NewReader(tc.input))
		if err != nil {
			t.Fatalf("%s: got error: %v", name, err)
		}
		got := buf.String()
		if got != tc.want {
			t.Fatalf("%s: got %v, expected: %v", name, got, tc.want)
		}
		if top != tc.top {
			t.Fatalf("%s: got top %v, expected: %v", name, top, tc.top)
		}
	}
}

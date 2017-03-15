// Copyright 2015 Google Inc. All Rights Reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package csslex

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strings"
	"testing"
)

const validCSS = `
/* comment **/
@import url('style.css') print;
body {
  background-color: white;
  color: #222
}
div p,
  #id:first-line {
    white-space: nowrap; }
@media print {
  body { font-size: 10pt }
}
.c1{color:red}.c2{color:blue}
body {
  background-image: url(data:image/png;base64,iVB);
}
`

func TestLex(t *testing.T) {
	expItems := []*Item{
		{ItemAtRuleIdent, 16, "@import"},
		{ItemAtRule, 24, "url('style.css') print"},
		{ItemSelector, 48, "body"},
		{ItemBlockStart, 53, ""},
		{ItemDecl, 57, "background-color: white"},
		{ItemDecl, 84, "color: #222"},
		{ItemBlockEnd, 96, ""},
		{ItemSelector, 98, "div p"},
		{ItemSelector, 107, "#id:first-line"},
		{ItemBlockStart, 122, ""},
		{ItemDecl, 128, "white-space: nowrap"},
		{ItemBlockEnd, 149, ""},
		{ItemAtRuleIdent, 151, "@media"},
		{ItemAtRuleBlockStart, 158, "print"},
		{ItemSelector, 168, "body"},
		{ItemBlockStart, 173, ""},
		{ItemDecl, 175, "font-size: 10pt"},
		{ItemBlockEnd, 191, ""},
		{ItemAtRuleBlockEnd, 194, ""},
		{ItemSelector, 195, ".c1"},
		{ItemBlockStart, 198, ""},
		{ItemDecl, 199, "color:red"},
		{ItemBlockEnd, 208, ""},
		{ItemSelector, 209, ".c2"},
		{ItemBlockStart, 212, ""},
		{ItemDecl, 213, "color:blue"},
		{ItemBlockEnd, 223, ""},
		{ItemSelector, 225, "body"},
		{ItemBlockStart, 230, ""},
		{ItemDecl, 234, "background-image: url(data:image/png;base64,iVB)"},
		{ItemBlockEnd, 284, ""},
	}
	i := 0
	for item := range Lex(validCSS) {
		if i > len(expItems)-1 {
			t.Errorf("%d: unexpected %+v", i, item)
		} else if !reflect.DeepEqual(item, expItems[i]) {
			t.Errorf("%d: item = %+v; want %+v", i, item, expItems[i])
		}
		i++
	}
	if i != len(expItems) {
		t.Errorf("len(items) = %d; want %d", i, len(expItems))
	}
}

func ExampleLex() {
	const cssText = `
	/* comment **/
	@import url('style.css') print;
	body {
	  background-color: white;
	  color: #222
	}
	div p,
	  #id:first-line {
	    white-space: nowrap; }
	@media print {
	  body { font-size: 10pt }
	}
	.c1{color:red}.c2{color:blue}
	`
	style := make(map[string]map[string]string)
	var skip bool
	var sel []string
	for item := range Lex(cssText) {
		switch item.Typ {
		case ItemError:
			log.Fatal(item)
		case ItemAtRuleIdent, ItemAtRule:
			continue
		case ItemAtRuleBlockStart:
			skip = true
			continue
		case ItemAtRuleBlockEnd:
			skip = false
			continue
		case ItemBlockEnd:
			sel = nil
		case ItemSelector:
			if skip {
				continue
			}
			sel = append(sel, item.Val)
			if _, ok := style[item.Val]; !ok {
				style[item.Val] = make(map[string]string)
			}
		case ItemDecl:
			if skip || len(sel) == 0 {
				continue
			}
			decl := strings.SplitN(item.Val, ":", 2)
			decl[0] = strings.TrimSpace(decl[0])
			decl[1] = strings.TrimSpace(decl[1])
			for _, s := range sel {
				style[s][decl[0]] = decl[1]
			}
		}
	}
	sb, err := json.MarshalIndent(style, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(sb))
	// Output:
	// {
	//   "#id:first-line": {
	//     "white-space": "nowrap"
	//   },
	//   ".c1": {
	//     "color": "red"
	//   },
	//   ".c2": {
	//     "color": "blue"
	//   },
	//   "body": {
	//     "background-color": "white",
	//     "color": "#222"
	//   },
	//   "div p": {
	//     "white-space": "nowrap"
	//   }
	// }
}
